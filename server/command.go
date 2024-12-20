package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const TRIGGER = "bulk-user-delete"
const USAGE = "[mode] [target users]"

const MODE_DRYRUN = "dry-run"
const MODE_LIVE = "live"

const USERS_INACTIVE = "inactive"
const USERS_ALL = "all"

func registerSlashCommand(client *pluginapi.Client) error {
	autocompleteData := model.NewAutocompleteData(TRIGGER, USAGE, "")
	autocompleteData.AddStaticListArgument("mode", true, []model.AutocompleteListItem{{
		Item:     MODE_DRYRUN,
		HelpText: "Simulate a bulk deletion. This will not change any data.",
	}, {
		Item:     MODE_LIVE,
		HelpText: "Perform a bulk deletion. This will change data.",
	}})
	autocompleteData.AddStaticListArgument("target users", true, []model.AutocompleteListItem{{
		Item:     USERS_INACTIVE,
		HelpText: "Only delete matching inactive users.",
	}, {
		Item:     USERS_ALL,
		HelpText: "Delete all matching users.",
	}})
	return client.SlashCommand.Register(&model.Command{
		Trigger:          TRIGGER,
		AutoComplete:     true,
		AutocompleteData: autocompleteData,
	})
}

func validateUser(client *pluginapi.Client, userid string) error {
	user, err := client.User.Get(userid)
	if err != nil {
		return fmt.Errorf("Could not retrieve running user context: %s", err.Error())
	}
	if !user.IsInRole(model.SystemAdminRoleId) {
		return fmt.Errorf("Only system administrators can run this command.")
	}
	return nil
}

func validateCommand(command string) error {
	fields := strings.Fields(command)
	if len(fields) != 3 {
		return fmt.Errorf("Missing argument. Usage: %s /%s", TRIGGER, USAGE)
	}
	if fields[0] != "/"+TRIGGER {
		return fmt.Errorf("Invalid command. Usage: %s /%s", TRIGGER, USAGE)
	}
	if fields[1] != MODE_DRYRUN && fields[1] != MODE_LIVE {
		return fmt.Errorf("Invalid mode. Must be '%s' or '%s'", MODE_DRYRUN, MODE_LIVE)
	}
	if fields[2] != USERS_INACTIVE && fields[2] != USERS_ALL {
		return fmt.Errorf("Invalid target users. Must be '%s' or '%s'", USERS_INACTIVE, USERS_ALL)
	}
	return nil
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	p.pluginClient.Log.Info("Bulk user deletion triggered", "user", args.UserId, "command", args.Command)

	if err := validateUser(p.pluginClient, args.UserId); err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         err.Error(),
		}, nil
	}

	if err := validateCommand(args.Command); err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         err.Error(),
		}, nil
	}

	fields := strings.Fields(args.Command)
	dryRun := fields[1] == MODE_DRYRUN
	targetInactiveUsersOnly := fields[2] == USERS_INACTIVE

	users, err := getUsers(p.pluginClient, targetInactiveUsersOnly)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unable to retrieve user list: %s", err.Error()),
		}, nil
	}

	config := p.getConfiguration()
	usersToDelete := filterForUsersByEmails(users, config.TargetEmailAddressSuffixes(), config.TargetEmailAddresses())

	if len(usersToDelete) == 0 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("There's nothing to do - there are no matching users to delete."),
		}, nil
	}

	var userList strings.Builder
	for _, user := range usersToDelete {
		fmt.Fprintf(&userList, "%s, ", user.Email)
	}
	userListString := strings.TrimSuffix(userList.String(), ", ")

	userListFileInfo, err := p.pluginClient.File.Upload(strings.NewReader(userListString),
		fmt.Sprintf("%d-target-users-bulk-delete-%s-%s.txt", time.Now().Unix(), fields[1], fields[2]), args.ChannelId)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unable to upload list of target users: %s", err.Error()),
		}, nil
	}

	go p.runBulkDeleteJob(dryRun, args.UserId, args.ChannelId, usersToDelete, userListFileInfo.Id)

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         fmt.Sprintf("Starting bulk user deletion job with command: `%s`", args.Command),
	}, nil
}
