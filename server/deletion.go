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

func purgeUsers(db *sql.DB, pluginClient *pluginapi.Client, socketClient *model.Client4, users []*model.User, reportProgress func(int)) error {
	for i, user := range users {
		resp, err := socketClient.PermanentDeleteUser(context.Background(), user.Id)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%d status code during attempt to delete user %s", resp.StatusCode, user.Email)
		}
		// There's a bug in `PermanentDeleteUser` that could result in
		// some user posts not getting deleted. So we go in after to
		// make sure all posts tied to this user are removed.
		if err := purgeHangingUserPosts(pluginClient, db, user.Id); err != nil {
			return err
		}
		pluginClient.Log.Info("Deleted user", "user", user.Email)
		reportProgress(i + 1)
	}
	return nil
}

func purgeHangingUserPosts(pluginClient *pluginapi.Client, db *sql.DB, userId string) error {
	for {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error when trying to begin transaction: %s", err.Error())
		}
		defer tx.Rollback()

		query := sq.Select("Id").
			From("Posts").
			Where(sq.Eq{"UserId": userId}).
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
			if err := rows.Scan(&id); err != nil {
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

		queryString, args, err = sq.Delete("Reactions").
			Where(sq.Eq{"postid": ids}).
			PlaceholderFormat(sq.Dollar).
			ToSql()
		if err != nil {
			return fmt.Errorf("error when trying to select user posts: %s", err.Error())
		}

		_, err = tx.Exec(queryString, args...)
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
	rows, err := db.Query(`
		SELECT id FROM Channels
		WHERE NOT EXISTS (
			SELECT 1
			FROM ChannelMembers
			WHERE ChannelMembers.channelid = Channels.id
		);
	`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var id string
		rows.Scan(&id)

		resp, err := socketClient.PermanentDeleteChannel(context.Background(), id)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%d status code during attempt to delete channel %s", resp.StatusCode, id)
		}
		pluginClient.Log.Info("Deleted channel", "channel", id)
	}

	return nil
}
