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
	}

	if len(userListFileInfoID) > 0 {
		statusPost.FileIds = model.StringArray{userListFileInfoID}
	}

	if dryRun {
		statusPost.Message = fmt.Sprintf("### Bulk user deletion job finished\nDry-run targeted %d users and all empty channels, boards, and playbooks", userCount)
		err := p.pluginClient.Post.CreatePost(statusPost)
		if err != nil {
			p.pluginClient.Log.Error("Unable to create status post", "error", err)
		}
		return
	}

	if err := p.pluginClient.Post.CreatePost(statusPost); err != nil {
		p.pluginClient.Log.Error("Bulk delete job unable to create status post. Aborting...")
		return
	}

	// Check if a job is already running, and if not mark it running
	set, err := p.pluginClient.KV.Set(RunningKey, true, pluginapi.SetAtomic(false))
	if err != nil {
		p.pluginClient.Log.Error("Could not determine if bulk delete job is already running. Aborting...", "error", err)
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"could not determine if a bulk delete job is already running. Aborting: %s", err.Error()), userCount, 0)
		return
	}
	if !set {
		p.pluginClient.Log.Warn("Bulk delete job is already running. Aborting...")
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"bulk delete job is already running - aborting"), userCount, 0)
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

		if status == userCount {
			statusPost.Message = fmt.Sprintf("### Bulk user deletion job started\nDeleted %d users. Cleaning up empty channels, boards, and playbooks...", userCount)
			if err = p.pluginClient.Post.UpdatePost(statusPost); err != nil {
				p.pluginClient.Log.Error("Unable to update status post", "error", err)
			}
			return
		}

		statusPost.Message = fmt.Sprintf("### Bulk user deletion job started\nDeleted %d/%d users...", status, userCount)
		if err = p.pluginClient.Post.UpdatePost(statusPost); err != nil {
			p.pluginClient.Log.Error("Unable to update status post", "error", err)
		}
	}); success {
		statusPost.Message = fmt.Sprintf("### Bulk user deletion job finished\nDeleted %d users and cleaned up empty channels, boards, and playbooks.", userCount)
		if err = p.pluginClient.Post.UpdatePost(statusPost); err != nil {
			p.pluginClient.Log.Error("Unable to update status post", "error", err)
		}
	}

	// Set the job not running
	_, err = p.pluginClient.KV.Set(RunningKey, false)
	if err != nil {
		p.pluginClient.Log.Error("Could not cleanup job status after run.", "error", err)
		reportError(p.pluginClient, statusPost, fmt.Errorf(
			"could not clean up job status after run: %s", err.Error()), userCount, userCount)
		return
	}
}

func bulkDelete(pluginClient *pluginapi.Client, socketClient *model.Client4, statusPost *model.Post, usersToDelete []*model.User, reportProgress func(int)) bool {
	db, err := pluginClient.Store.GetMasterDB()
	if err != nil {
		pluginClient.Log.Error("Error accessing database", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf(
			"error accessing database to find empty channels: %s", err.Error()), len(usersToDelete), 0)
		return false
	}

	// Delete the specified users and all related user data.
	if count, err := purgeUsers(db, pluginClient, socketClient, usersToDelete, reportProgress); err != nil {
		pluginClient.Log.Error("Error deleting users", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf(
			"error deleting users: %s", err.Error()), len(usersToDelete), count)
		return false
	}

	// Delete board members that no longer exist in the user table
	if err := purgeDanglingBoardMembers(db); err != nil {
		pluginClient.Log.Error("Error removing users from board members list", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing users from board members list: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete boards that have no members
	if err := purgeEmptyBoards(db, pluginClient); err != nil {
		pluginClient.Log.Error("Error removing empty boards", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing empty boards: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete playbook members that no longer exist in the user table
	if err := purgeDanglingPlaybookMembers(db); err != nil {
		pluginClient.Log.Error("Error removing users from playbook members list", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing users from playbook members list: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete playbook runs with no members
	if err := purgeRunsForEmptyPlaybooks(db); err != nil {
		pluginClient.Log.Error("Error removing empty playbook runs", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing empty playbook runs: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete playbooks with no members
	if err := purgeEmptyPlaybooks(db); err != nil {
		pluginClient.Log.Error("Error removing empty playbooks", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing empty playbooks: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete miscellaneous data related to deleted playbooks
	if err := purgeDanglingPlaybookData(db); err != nil {
		pluginClient.Log.Error("Error removing dangling playbook data", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error removing dangling playbook data: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	// Delete all empty channels. The expectation is that empty channels were
	// channels that previously had users in them - we just deleted them.
	if err := purgeEmptyChannels(db, pluginClient, socketClient); err != nil {
		pluginClient.Log.Error("Error deleting empty channels", "error", err)
		reportError(pluginClient, statusPost, fmt.Errorf("error deleting empty channels: %s", err.Error()), len(usersToDelete), len(usersToDelete))
		return false
	}

	pluginClient.Log.Info("Finished bulk deletion", "userDeletionCount", len(usersToDelete))
	return true
}

func reportError(pluginClient *pluginapi.Client, statusPost *model.Post, err error, totalDeletionCount, currDeletionCount int) {
	statusPost.Message = fmt.Sprintf("### Bulk user deletion job failed!\n%s\nDeleted %d/%d users.", err.Error(), currDeletionCount, totalDeletionCount)
	if err := pluginClient.Post.UpdatePost(statusPost); err != nil {
		pluginClient.Log.Error("Unable to update status post", "error", err)
	}
}
