package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

func purgeUsers(db *sql.DB, pluginClient *pluginapi.Client, socketClient *model.Client4, users []*model.User) error {
	for _, user := range users {
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
		if err := purgeHangingUserPosts(db, user.Id); err != nil {
			return err
		}
		pluginClient.Log.Info("Deleted user", "user", user.Email)
	}
	return nil
}

func purgeHangingUserPosts(db *sql.DB, userId string) error {
	// Delete threads related to User's posts
	_, err := db.Exec(fmt.Sprintf(`
DELETE FROM threads
  WHERE postid IN (
    SELECT id
    FROM posts
      WHERE userid = '%s'
  );
`, userId))
	if err != nil {
		return fmt.Errorf("error when trying to delete user threads: %s", err.Error())
	}

	// Delete thread memberships related to User's posts
	_, err = db.Exec(fmt.Sprintf(`
DELETE FROM threadmemberships
  WHERE postid IN (
    SELECT id
    FROM posts
      WHERE posts.userid = '%s'
  );
`, userId))
	if err != nil {
		return fmt.Errorf("error when trying to delete user thread members: %s", err.Error())
	}

	// Delete reactions related to User's posts
	_, err = db.Exec(fmt.Sprintf(`
DELETE FROM reactions
  WHERE postid IN (
    SELECT id
    FROM posts
      WHERE posts.userid = '%s'
  );
`, userId))
	if err != nil {
		return fmt.Errorf("error when trying to delete user post reactions: %s", err.Error())
	}

	// Delete replies to User's posts
	_, err = db.Exec(fmt.Sprintf(`
DELETE FROM posts
  WHERE rootid IN (
    SELECT id
    FROM posts
      WHERE posts.userid = '%s'
  );
`, userId))
	if err != nil {
		return fmt.Errorf("error when trying to delete replies to user posts: %s", err.Error())
	}

	// Delete User's posts
	_, err = db.Exec(fmt.Sprintf(`
DELETE FROM posts
  WHERE posts.userid = '%s';
`, userId))
	if err != nil {
		return fmt.Errorf("error when trying to delete user posts: %s", err.Error())
	}

	return nil
}

func purgeEmptyChannels(db *sql.DB, pluginClient *pluginapi.Client, socketClient *model.Client4) error {
	rows, err := db.Query(`
SELECT id FROM channels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = channels.id
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
