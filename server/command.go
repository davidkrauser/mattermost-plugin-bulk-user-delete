package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const Trigger = "bulk-user-delete"
const Usage = "[mode] [target users]"

const ModeDryRun = "dry-run"
const ModeLive = "live"

const UsersInactive = "inactive"
const UsersAll = "all"

func registerSlashCommand(client *pluginapi.Client) error {
	autocompleteData := model.NewAutocompleteData(Trigger, Usage, "")
	autocompleteData.RoleID = model.SystemAdminRoleId
	autocompleteData.AddStaticListArgument("mode", true, []model.AutocompleteListItem{{
		Item:     ModeDryRun,
		HelpText: "Simulate a bulk deletion. This will not change any data.",
	}, {
		Item:     ModeLive,
		HelpText: "Perform a bulk deletion. This will change data.",
	}})
	autocompleteData.AddStaticListArgument("target users", true, []model.AutocompleteListItem{{
		Item:     UsersInactive,
		HelpText: "Only delete matching inactive users.",
	}, {
		Item:     UsersAll,
		HelpText: "Delete all matching users.",
	}})
	return client.SlashCommand.Register(&model.Command{
		Trigger:          Trigger,
		AutoComplete:     true,
		AutocompleteData: autocompleteData,
	})
}

func validateUser(client *pluginapi.Client, userid string) error {
	user, err := client.User.Get(userid)
	if err != nil {
		return fmt.Errorf("could not retrieve running user context: %s", err.Error())
	}
	if !user.IsInRole(model.SystemAdminRoleId) {
		return fmt.Errorf("only system administrators can run this command")
	}
	return nil
}

func validateCommand(command string) error {
	fields := strings.Fields(command)
	if len(fields) != 3 {
		return fmt.Errorf("missing argument. Usage: %s /%s", Trigger, Usage)
	}
	if fields[0] != "/"+Trigger {
		return fmt.Errorf("invalid command. Usage: %s /%s", Trigger, Usage)
	}
	if fields[1] != ModeDryRun && fields[1] != ModeLive {
		return fmt.Errorf("invalid mode. Must be '%s' or '%s'", ModeDryRun, ModeLive)
	}
	if fields[2] != UsersInactive && fields[2] != UsersAll {
		return fmt.Errorf("invalid target users. Must be '%s' or '%s'", UsersInactive, UsersAll)
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
	dryRun := fields[1] == ModeDryRun
	targetInactiveUsersOnly := fields[2] == UsersInactive

	users, err := getUsers(p.pluginClient, targetInactiveUsersOnly)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unable to retrieve user list: %s", err.Error()),
		}, nil
	}

	config := p.getConfiguration()
	usersToDelete := filterForUsersByEmails(p.pluginClient, users, config.TargetEmailAddressSuffixes(), config.TargetEmailAddresses())

	if len(usersToDelete) == 0 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "There's nothing to do - there are no matching users to delete.",
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
