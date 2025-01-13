package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

func purgeDanglingBoardMembers(db *sql.DB) error {
	_, err := db.Exec(`
			DELETE FROM focalboard_board_members
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM Users
			      WHERE Users.id = focalboard_board_members.user_id
			  ) AND NOT focalboard_board_members.user_id = 'system';
		`)
	if err != nil {
		return err
	}
	return nil
}

func purgeEmptyBoards(db *sql.DB, pluginClient *pluginapi.Client) error {
	rows, err := db.Query(`
			SELECT id FROM focalboard_boards
			  WHERE NOT EXISTS (
			    SELECT 1
			    FROM focalboard_board_members
			      WHERE focalboard_board_members.board_id = focalboard_boards.id
			  );
		`)
	if err != nil {
		return fmt.Errorf("error when trying to find empty boards: %s", err.Error())
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("error parsing board IDs: %s", scanErr.Error())
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		err = deleteBoard(db, pluginClient, id)
		if err != nil {
			return fmt.Errorf("error deleting board: %s", err.Error())
		}
	}
	return nil
}

func deleteBoard(db *sql.DB, pluginClient *pluginapi.Client, boardID string) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error when trying to begin transaction: %s", err.Error())
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			err = fmt.Errorf("error when trying to rollback transaction: %s on error: %s",
				rollbackErr.Error(), err.Error())
		}
	}()

	blockDeleteQuery := sq.Delete("focalboard_blocks").
		Where(sq.Eq{"board_id": boardID}).
		PlaceholderFormat(sq.Dollar)

	blockDeleteQueryString, blockDeleteQueryArgs, err := blockDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete block: %s", err.Error())
	}

	_, err = tx.Exec(blockDeleteQueryString, blockDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete block: %s", err.Error())
	}

	deleteBlocksHistoryQuery := sq.Delete("focalboard_blocks_history").
		Where(sq.Eq{"board_id": boardID}).
		PlaceholderFormat(sq.Dollar)

	deleteBlockHistoryQueryString, deleteBlockHistoryArgs, err := deleteBlocksHistoryQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete block history query: %s", err.Error())
	}

	_, err = tx.Exec(deleteBlockHistoryQueryString, deleteBlockHistoryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete block history: %s", err.Error())
	}

	boardDeleteQuery := sq.Delete("focalboard_boards").
		Where(sq.Eq{"id": boardID}).
		PlaceholderFormat(sq.Dollar)

	boardDeleteQueryString, boardDeleteQueryArgs, err := boardDeleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete board: %s", err.Error())
	}

	_, err = tx.Exec(boardDeleteQueryString, boardDeleteQueryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete board: %s", err.Error())
	}

	deleteBoardsHistoryQuery := sq.Delete("focalboard_boards_history").
		Where(sq.Eq{"id": boardID}).
		PlaceholderFormat(sq.Dollar)

	deleteBoardsHistoryQueryString, deleteBoardsHistoryArgs, err := deleteBoardsHistoryQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete board history query: %s", err.Error())
	}

	_, err = tx.Exec(deleteBoardsHistoryQueryString, deleteBoardsHistoryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete board history: %s", err.Error())
	}

	fileInfosToDelete, err := tx.Query(`
		SELECT id, path FROM fileinfo
		WHERE creatorid = 'boards' AND
		    NOT EXISTS (
		        SELECT 1 FROM focalboard_blocks
		        WHERE NOT fields->>'fileId' = '' AND
                            fileinfo.path LIKE CONCAT('%', fields->>'fileId')
		    );
	`)
	if err != nil {
		return fmt.Errorf("error when trying to find board files: %s", err.Error())
	}
	defer fileInfosToDelete.Close()

	fileSettings := pluginClient.Configuration.GetConfig().FileSettings
	if fileSettings.DriverName == nil || *fileSettings.DriverName != "local" {
		return fmt.Errorf("only local storage file drivers are supported")
	}

	var fileInfoIDs []string
	for fileInfosToDelete.Next() {
		var id, path string
		if err = fileInfosToDelete.Scan(&id, &path); err != nil {
			return fmt.Errorf("error when parsing file info IDs: %s", err.Error())
		}
		fileInfoIDs = append(fileInfoIDs, id)

		exists, existsErr := fileExists(filepath.Join(*fileSettings.Directory, path))
		if existsErr != nil {
			return fmt.Errorf("error when trying to determine if file exists: %s", existsErr.Error())
		}
		if !exists {
			pluginClient.Log.Warn("tried to delete a file that doesn't exist", "id", id, "path", path)
			continue
		}

		err = removeFile(filepath.Join(*fileSettings.Directory, path))
		if err != nil {
			return fmt.Errorf("error trying to delete file: %s", err.Error())
		}
	}

	deleteFileInfosQuery := sq.Delete("fileinfo").
		Where(sq.Eq{"id": fileInfoIDs}).
		PlaceholderFormat(sq.Dollar)

	deleteFileInfosQueryString, deleteFileInfosArgs, err := deleteFileInfosQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the query to delete file info: %s", err.Error())
	}

	_, err = tx.Exec(deleteFileInfosQueryString, deleteFileInfosArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete file info: %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
	}

	return nil
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil {
		return errors.Wrapf(err, "unable to remove the file %s", path)
	}
	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "unable to know if file %s exists", path)
	}
	return true, nil
}
