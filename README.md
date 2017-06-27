# Mattermost-Slack Mirroring Bot

Mirrors chats between Mattermost and Slack, both ways. Written in golang. Only works for public channels which exist on both sides.
The bot never creates a channel on its own.

## Usage

- Create a user for the bot on the Mattermost server, add it to the relevant team.
- Create a bot user for Slack.
- The bot joins all public channels on the Mattermost side automatically but on the Slack side
  you'll need to invite the bot manually to each channel you want to mirror.
- Run the bot:

```
$ git clone https://github.com/nilenso/mattermost-slack-mirror-bot.git
$ cd mattermost-slack-mirror-bot
$ glide install
$ go build
$ ./mattermost-slack-mirror-bot <mm_server_host> <mm_team> <mm_bot_user_email> <mm_bot_user_password> <slack_token> <timezone_location>
```

`timezone_location` should be a value from the IANA Time Zone database like "Asia/Kolkata".

- Channels are matched by their names.
- Users are matched by their emails.
- Slack messages posted by the bot appear to be posted by the matching user.
  But Mattermost messages posted by the bot bear the bot's name.
