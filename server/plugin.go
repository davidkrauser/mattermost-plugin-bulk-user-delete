package main

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const SOCKETCLIENTPATH = "/var/tmp/mattermost_local.socket"
const RUNLOCKKEY = "com.mattermost.plugin-bulk-user-delete/runlock"
const RUNNINGKEY = "com.mattermost.plugin-bulk-user-delete/running"

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
	// Some needed APIs are only available over the REST API,
	// so we'll hit them over a local socket.
	p.socketClient = model.NewAPIv4SocketClient(SOCKETCLIENTPATH)
	p.pluginClient = pluginapi.NewClient(p.API, p.Driver)

	if _, err := p.pluginClient.KV.Set(RUNNINGKEY, false); err != nil {
		return err
	}

	return registerSlashCommand(p.pluginClient)
}
