# Bulk User Delete Plugin

This plugin can be used to permanently remove users from a running Mattermost instance.

It introduces a new slash command, `/bulk-user-delete` to start the removal:

<img alt="screenshot-slash-command" src="images/screenshot-slash-command.png" style="width:60%; height:auto" >

The removal is configured in the plugin settings. Only users with email addresses that match these filters will be removed by the slash command:

<img alt="screenshot-settings" src="images/screenshot-settings.png" style="width:60%; height:auto" >

Before actually permanently removing users, you can run the command in dry-run mode to see what will get removed:

<img alt="screenshot-dry-run-all" src="images/screenshot-dry-run-all.png" style="width:60%; height:auto" >

The slash command can be used to remove all users that match the specified email address filters. Alternatively, it can be used to only remove inactive users. If we deactivate a few users:

<img alt="screenshot-deactivate-user" src="images/screenshot-deactivate-user.png" style="width:60%; height:auto" >

We can now remove only those users with the slash command:

<img alt="screenshot-live-delete-inactive" src="images/screenshot-live-delete-inactive.png" style="width:60%; height:auto" >

## Limitations

This plugin does not currently remove user data for external plugins. For example, any data created with the popular [boards](https://github.com/mattermost/mattermost-plugin-boards) or [playbooks](https://github.com/mattermost/mattermost-plugin-playbooks) plugins will not have user data removed.

Additionally, the SQL queries used in this plugin to remove data are not optimized for performance, but instead for simplicity. The hope is that simpler queries are easier to understand and less likely to have errors.
