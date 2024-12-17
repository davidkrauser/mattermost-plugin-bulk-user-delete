package main

import (
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const JOBKEY = "com.mattermost.plugin-bulk-user-delete/maintenance-job"

func (p *Plugin) scheduleMaintenanceJob() error {
	_, err := cluster.Schedule(p.API, JOBKEY, cluster.MakeWaitForInterval(time.Minute), func() {
		run(p.pluginClient, p.socketClient, p.getConfiguration())
	})
	if err != nil {
		return err
	}
	return nil
}

func run(pluginClient *pluginapi.Client, socketClient *model.Client4, config *configuration) {
	pluginClient.Log.Info("running")

	targetEmailAddresses := config.TargetEmailAddresses()
	targetEmailAddressSuffixes := config.TargetEmailAddressSuffixes()

	if err := purgeUsers(pluginClient, socketClient, config.TargetInactiveUsersOnly, targetEmailAddressSuffixes, targetEmailAddresses); err != nil {
		pluginClient.Log.Error("error deleting users", "error", err)
		return
	}

	db, err := pluginClient.Store.GetMasterDB()
	if err != nil {
		pluginClient.Log.Error("error accessing database", "error", err)
		return
	}

	if err := purgeEmptyChannels(db, pluginClient); err != nil {
		pluginClient.Log.Error("error deleting empty channels", "error", err)
		return
	}
}
