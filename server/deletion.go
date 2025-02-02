package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	_ "github.com/lib/pq"
)

func purgeUsers(db *sql.DB, pluginClient *pluginapi.Client, socketClient *model.Client4, users []*model.User, reportProgress func(int)) (int, error) {
	for i, user := range users {
		resp, err := socketClient.PermanentDeleteUser(context.Background(), user.Id)
		if err != nil {
			return i, err
		}
		if resp.StatusCode != http.StatusOK {
			return i, fmt.Errorf("%d status code during attempt to delete user %s", resp.StatusCode, user.Email)
		}
		// There's a bug in `PermanentDeleteUser` that could result in
		// some user posts not getting deleted. So we go in after to
		// make sure all posts tied to this user are removed.
		if err := purgeDanglingUserPosts(db, user.Id); err != nil {
			return i, fmt.Errorf("error trying to purge dangling posts for user %s: %s", user.Id, err.Error())
		}
		// `PermanentDeleteUser` misses a few tables with user data.
		// We take an extra step here to ensure that data is removed.
		if err := purgeDanglingUserData(db, user.Id); err != nil {
			return i, fmt.Errorf("error trying to purge dangling metadata for user %s: %s", user.Id, err.Error())
		}
		pluginClient.Log.Info("Deleted user", "user", user.Email)
		reportProgress(i + 1)
	}
	return len(users), nil
}

func purgeDanglingUserData(db *sql.DB, userID string) (err error) {
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

	deleteStatusQuery := sq.Delete("Status").
		Where(sq.Eq{"userid": userID}).
		PlaceholderFormat(sq.Dollar)

	deleteStatusQueryString, deleteStatusArgs, err := deleteStatusQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete status query: %s", err.Error())
	}

	_, err = tx.Exec(deleteStatusQueryString, deleteStatusArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete user status: %s", err.Error())
	}

	deleteChannelMemberHistoryQuery := sq.Delete("ChannelMemberHistory").
		Where(sq.Eq{"userid": userID}).
		PlaceholderFormat(sq.Dollar)

	deleteChannelMemberHistoryQueryString, deleteChannelMemberHistoryArgs, err := deleteChannelMemberHistoryQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete channel member history query: %s", err.Error())
	}

	_, err = tx.Exec(deleteChannelMemberHistoryQueryString, deleteChannelMemberHistoryArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete user channel member history: %s", err.Error())
	}

	deleteSidebarCategoriesQuery := sq.Delete("SidebarCategories").
		Where(sq.Eq{"userid": userID}).
		PlaceholderFormat(sq.Dollar)

	deleteSidebarCategoriesQueryString, deleteSidebarCategoriesArgs, err := deleteSidebarCategoriesQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete sidebar categories query: %s", err.Error())
	}

	_, err = tx.Exec(deleteSidebarCategoriesQueryString, deleteSidebarCategoriesArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete user sidebar categories: %s", err.Error())
	}

	deleteProductNoticeViewStateQuery := sq.Delete("ProductNoticeViewState").
		Where(sq.Eq{"userid": userID}).
		PlaceholderFormat(sq.Dollar)

	deleteProductNoticeViewStateQueryString, deleteProductNoticeViewStateArgs, err := deleteProductNoticeViewStateQuery.ToSql()
	if err != nil {
		return fmt.Errorf("error when trying to build the delete product notice view state query: %s", err.Error())
	}

	_, err = tx.Exec(deleteProductNoticeViewStateQueryString, deleteProductNoticeViewStateArgs...)
	if err != nil {
		return fmt.Errorf("error when trying to delete user product notice view state: %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
	}

	return nil
}

func purgeDanglingUserPosts(db *sql.DB, userID string) (err error) {
	for {
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

		query := sq.Select("Id").
			From("Posts").
			Where(sq.Eq{"UserId": userID}).
			Limit(1000).
			PlaceholderFormat(sq.Dollar)

		queryString, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to build the query: %s", err.Error())
		}

		rows, err := tx.Query(queryString, args...)
		if err != nil {
			return fmt.Errorf("error when trying to select user posts: %s", err.Error())
		}
		defer rows.Close()

		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				return fmt.Errorf("error when scanning row: %s", err.Error())
			}
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			break
		}

		deleteThreadsQuery := sq.Delete("Threads").
			Where(sq.Eq{"postid": ids}).
			PlaceholderFormat(sq.Dollar)

		deleteThreadsQueryString, deleteThreadsArgs, err := deleteThreadsQuery.ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to build the delete threads query: %s", err.Error())
		}

		_, err = tx.Exec(deleteThreadsQueryString, deleteThreadsArgs...)
		if err != nil {
			return fmt.Errorf("error when trying to delete user threads: %s", err.Error())
		}

		deleteThreadMembershipsQuery := sq.Delete("ThreadMemberships").
			Where(sq.Eq{"postid": ids}).
			PlaceholderFormat(sq.Dollar)

		deleteThreadMembershipsQueryString, deleteThreadMembershipsArgs, err := deleteThreadMembershipsQuery.ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to build the delete thread memberships query: %s", err.Error())
		}

		_, err = tx.Exec(deleteThreadMembershipsQueryString, deleteThreadMembershipsArgs...)
		if err != nil {
			return fmt.Errorf("error when trying to delete user thread memberships: %s", err.Error())
		}

		deleteReactionsQueryString, deleteReactionsArgs, err := sq.Delete("Reactions").
			Where(sq.Eq{"postid": ids}).
			PlaceholderFormat(sq.Dollar).
			ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to select user posts: %s", err.Error())
		}

		_, err = tx.Exec(deleteReactionsQueryString, deleteReactionsArgs...)
		if err != nil {
			return fmt.Errorf("error when trying to delete user post reactions: %s", err.Error())
		}

		// Delete comments under posts
		deletePostsQuery := sq.Delete("Posts").
			Where(sq.Eq{"rootid": ids}).
			PlaceholderFormat(sq.Dollar)

		deletePostsQueryString, deletePostsArgs, err := deletePostsQuery.ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to build the delete posts query: %s", err.Error())
		}

		_, err = tx.Exec(deletePostsQueryString, deletePostsArgs...)
		if err != nil {
			return fmt.Errorf("error when trying to delete replies to user posts: %s", err.Error())
		}

		deletePostsQuery = sq.Delete("Posts").
			Where(sq.Eq{"Id": ids}).
			PlaceholderFormat(sq.Dollar)

		deletePostsQueryString, deletePostsArgs, err = deletePostsQuery.ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to build the delete posts query: %s", err.Error())
		}

		_, err = tx.Exec(deletePostsQueryString, deletePostsArgs...)
		if err != nil {
			return fmt.Errorf("error when trying to delete replies to user posts: %s", err.Error())
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("error when trying to commit the transaction: %s", err.Error())
		}
	}

	return nil
}

func purgeEmptyChannels(db *sql.DB, pluginClient *pluginapi.Client, socketClient *model.Client4) error {
	for {
		rows, err := db.Query(`
			SELECT id FROM Channels
			WHERE type NOT IN ('D', 'G')
			    AND NOT EXISTS (
			        SELECT 1
			        FROM ChannelMembers INNER JOIN Users on ChannelMembers.userid = Users.id
			        WHERE ChannelMembers.channelid = Channels.id AND Users.username != 'playbooks'
			    )
			LIMIT 1000;
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			break
		}

		for _, id := range ids {
			resp, err := socketClient.PermanentDeleteChannel(context.Background(), id)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%d status code during attempt to delete channel %s", resp.StatusCode, id)
			}
			pluginClient.Log.Info("Deleted channel", "channel", id)
		}
	}

	return nil
}
