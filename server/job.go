package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

func (p *Plugin) runBulkDeleteJob(dryRun bool, runningUserID string, runningChannelID string, usersToDelete []*model.User, userListFileInfoID string) {
	userCount := len(usersToDelete)
	statusPost := &model.Post{
		UserId:    runningUserID,
		ChannelId: runningChannelID,
		Message:   fmt.Sprintf("### Bulk user deletion job started\nDeleting %d users...", userCount),
		FileIds:   model.StringArray{userListFileInfoID},
	}

	if dryRun {
		statusPost.Message = fmt.Sprintf("### (DRY RUN) Bulk user deletion job finished\nDeleted %d users", userCount)
		err := p.pluginClient.Post.CreatePost(statusPost)
		if err != nil {
			p.pluginClient.Log.Error("Unable to create status post", "error", err)
		}
		return
	}

	if err := p.pluginClient.Post.CreatePost(statusPost); err != nil {
		p.pluginClient.Log.Info("Bulk delete job unable to create status post. Aborting...")
		return
	}

	// Check if a job is already running, and if not mark it running
	set, err := p.pluginClient.KV.Set(RunningKey, true, pluginapi.SetAtomic(false))
	if err != nil {
		p.pluginClient.Log.Error("Could not determine if bulk delete job is already running. Aborting...", "error", err)
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"could not determine if a bulk delete job is already running. Aborting: %s", err.Error()))
		return
	}
	if !set {
		p.pluginClient.Log.Info("Bulk delete job is already running. Aborting...")
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"bulk delete job is already running - aborting"))
		return
	}

	lastTime := time.Now()
	if success := bulkDelete(p.pluginClient, p.socketClient, statusPost, usersToDelete, func(status int) {
		currTime := time.Now()
		elapsed := currTime.Sub(lastTime)
		// Only update the status post once per second
		if elapsed < time.Second {
			return
		}
		lastTime = currTime

		statusPost.Message = fmt.Sprintf("### Bulk user deletion job started\nDeleted %d/%d users...", status, userCount)
		if err = p.pluginClient.Post.UpdatePost(statusPost); err != nil {
			p.pluginClient.Log.Error("Unable to update status post", "error", err)
		}
	}); success {
		statusPost.Message = fmt.Sprintf("### Bulk user deletion job finished\nDeleted %d users", userCount)
		if err = p.pluginClient.Post.UpdatePost(statusPost); err != nil {
			p.pluginClient.Log.Error("Unable to update status post", "error", err)
		}
	}

	// Set the job not running
	_, err = p.pluginClient.KV.Set(RunningKey, false)
	if err != nil {
		p.pluginClient.Log.Error("Could not cleanup job status after run.", "error", err)
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"could not clean up job status after run: %s", err.Error()))
		return
	}
}

func bulkDelete(pluginClient *pluginapi.Client, socketClient *model.Client4, statusPost *model.Post, usersToDelete []*model.User, reportProgress func(int)) bool {
	db, err := pluginClient.Store.GetMasterDB()
	if err != nil {
		pluginClient.Log.Error("Error accessing database", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf(
			"error accessing database to find empty channels: %s", err.Error()))
		return false
	}

	// Delete the specified users and all related user data.
	if err := purgeUsers(db, pluginClient, socketClient, usersToDelete, reportProgress); err != nil {
		pluginClient.Log.Error("Error deleting users", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf(
			"error deleting users: %s", err.Error()))
		return false
	}

	// Delete all empty channels. The expectation is that empty channels were
	// channels that previously had users in them - we just deleted them.
	if err := purgeEmptyChannels(db, pluginClient, socketClient); err != nil {
		pluginClient.Log.Error("Error deleting empty channels", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error deleting empty channels: %s", err.Error()))
		return false
	}

	pluginClient.Log.Info("Finished bulk deletion", "userDeletionCount", len(usersToDelete))
	return true
}

func reportError(pluginClient *pluginapi.Client, statusPost *model.Post, err error) {
	statusPost.Message = fmt.Sprintf("### Bulk user deletion job failed!\n%s", err.Error())
	if err := pluginClient.Post.UpdatePost(statusPost); err != nil {
		pluginClient.Log.Error("Unable to update status post", "error", err)
	}
}
