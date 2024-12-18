package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

func runBulkDeleteJob(runLock *cluster.Mutex, pluginClient *pluginapi.Client, socketClient *model.Client4, runningUserId string, runningChannelId string, usersToDelete []*model.User) {
	// Ensure only one job runs at a time
	runLock.Lock()
	defer runLock.Unlock()

	db, err := pluginClient.Store.GetMasterDB()
	if err != nil {
		pluginClient.Log.Error("Error accessing database", "error", err)
		reportError(pluginClient, runningUserId, runningChannelId, fmt.Errorf(
			"Error accessing database to find empty channels: %s", err.Error()))
		return
	}

	// Delete the specified users and all related user data.
	if err := purgeUsers(db, pluginClient, socketClient, usersToDelete); err != nil {
		pluginClient.Log.Error("Error deleting users", "error", err)
		reportError(pluginClient, runningUserId, runningChannelId, fmt.Errorf(
			"Error deleting users: %s", err.Error()))
		return
	}

	// Delete all empty channels. The expectation is that empty channels were
	// channels that previously had users in them - we just deleted them.
	if err := purgeEmptyChannels(db, pluginClient, socketClient); err != nil {
		pluginClient.Log.Error("Error deleting empty channels", "error", err)
		reportError(pluginClient, runningUserId, runningChannelId, fmt.Errorf(
			"Error deleting empty channels: %s", err.Error()))
		return
	}

	userDeletionCount := len(usersToDelete)
	pluginClient.Log.Info("Finished bulk deletion", "userDeletionCount", userDeletionCount)
	reportSuccess(pluginClient, runningUserId, runningChannelId, userDeletionCount)
}

func reportError(pluginClient *pluginapi.Client, runningUserId string, runningChannelId string, err error) {
	pluginClient.Post.CreatePost(&model.Post{
		UserId:    runningUserId,
		ChannelId: runningChannelId,
		Message:   fmt.Sprintf("**Bulk delete job failed!** %s", err.Error()),
	})
}

func reportSuccess(pluginClient *pluginapi.Client, runningUserId string, runningChannelId string, userDeletionCount int) {
	pluginClient.Post.CreatePost(&model.Post{
		UserId:    runningUserId,
		ChannelId: runningChannelId,
		Message:   fmt.Sprintf("**Bulk deletion job complete!** %d users deleted.", userDeletionCount),
	})
}
