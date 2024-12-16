package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

func getUsers(client *pluginapi.Client, targetInactiveOnly bool) ([]*model.User, error) {
	options := model.UserGetOptions{
		PerPage: 100,
	}
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

func filterForUsersByEmails(users []*model.User, targetEmailSuffixes, targetEmailAddresses []string) []*model.User {
	var usersToDelete []*model.User
	for _, user := range users {
		// We can't permanently delete system administrators
		if user.IsInRole(model.SystemAdminRoleId) {
			continue
		}
		if emailMatches(user, targetEmailSuffixes, targetEmailAddresses) {
			usersToDelete = append(usersToDelete, user)
		}
	}
	return usersToDelete
}

func emailMatches(user *model.User, targetEmailSuffixes, targetEmailAddresses []string) bool {
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
