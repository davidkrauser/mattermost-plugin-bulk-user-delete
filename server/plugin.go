package main

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const SOCKETCLIENTPATH = "/var/tmp/mattermost_local.socket"

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	pluginClient *pluginapi.Client
	socketClient *model.Client4

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

// OnActivate is invoked when the plugin is activated.
func (p *Plugin) OnActivate() error {
	p.pluginClient = pluginapi.NewClient(p.API, p.Driver)
	p.socketClient = model.NewAPIv4SocketClient(SOCKETCLIENTPATH)
	return p.scheduleMaintenanceJob()
}
