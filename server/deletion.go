package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

func purgeUsers(pluginClient *pluginapi.Client, socketClient *model.Client4, targetInactiveOnly bool, targetEmailSuffixes, targetEmailAddresses []string) error {
	users, err := getUsers(pluginClient, targetInactiveOnly)
	if err != nil {
		return err
	}
	usersToDelete := filterForUsersToDelete(users, targetEmailSuffixes, targetEmailAddresses)
	for _, user := range usersToDelete {
		resp, err := socketClient.PermanentDeleteUser(context.TODO(), user.Id)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%d status code during attempt to delete %s", resp.StatusCode, user.Email)
		}
		pluginClient.Log.Info("deleted user", "user", user.Email)
	}
	return nil
}

func getUsers(client *pluginapi.Client, targetInactiveOnly bool) ([]*model.User, error) {
	var options model.UserGetOptions
	options.PerPage = 100
	if targetInactiveOnly {
		options.Inactive = true
	}
	var users []*model.User
	for {
		page, err := client.User.List(&options)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		users = append(users, page...)
		options.Page++
	}
	return users, nil
}

func filterForUsersToDelete(users []*model.User, targetEmailSuffixes, targetEmailAddresses []string) []*model.User {
	var usersToDelete []*model.User
	for _, user := range users {
		if shouldDelete(user, targetEmailSuffixes, targetEmailAddresses) {
			usersToDelete = append(usersToDelete, user)
		}
	}
	return usersToDelete
}

func shouldDelete(user *model.User, targetEmailSuffixes, targetEmailAddresses []string) bool {
	for _, suffix := range targetEmailSuffixes {
		if strings.HasSuffix(user.Email, suffix) {
			return true
		}
	}
	for _, exact := range targetEmailAddresses {
		if user.Email == exact {
			return true
		}
	}
	return false
}

func purgeEmptyChannels(db *sql.DB, pluginClient *pluginapi.Client) error {
	query := `
DELETE FROM channelbookmarks
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = channelbookmarks.channelid
  );

DELETE FROM channelmemberhistory
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = channelmemberhistory.channelid
  );

DELETE FROM commandwebhooks
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = commandwebhooks.channelid
  );

DELETE FROM drafts
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = drafts.channelid
  );

DELETE FROM fileinfo
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = fileinfo.channelid
  );

DELETE FROM groupchannels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = groupchannels.channelid
  );

DELETE FROM incomingwebhooks
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = incomingwebhooks.channelid
  );

DELETE FROM outgoingwebhooks
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = outgoingwebhooks.channelid
  );

DELETE FROM posts
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = posts.channelid
  );

DELETE FROM postspriority
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = postspriority.channelid
  );

DELETE FROM reactions
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = reactions.channelid
  );

DELETE FROM retentionpolicieschannels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = retentionpolicieschannels.channelid
  );

DELETE FROM scheduledposts
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = scheduledposts.channelid
  );

DELETE FROM sharedchannelremotes
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = sharedchannelremotes.channelid
  );

DELETE FROM sharedchannels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = sharedchannels.channelid
  );

DELETE FROM sharedchannelusers
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = sharedchannelusers.channelid
  );

DELETE FROM sidebarchannels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = sidebarchannels.channelid
  );

DELETE FROM threads
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = threads.channelid
  );

DELETE FROM uploadsessions
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = uploadsessions.channelid
  );

DELETE FROM publicchannels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = publicchannels.id
  );

DELETE FROM channels
  WHERE NOT EXISTS (
    SELECT 1
    FROM channelmembers
      WHERE channelmembers.channelid = channels.id
  );
`
	result, err := db.Exec(query)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		pluginClient.Log.Warn("maybe deleted channels, but could not determine how many")
		return nil
	}

	if rowsAffected == 0 {
		return nil
	}

	pluginClient.Log.Info("deleted channels", "count", rowsAffected)
	return nil
}
