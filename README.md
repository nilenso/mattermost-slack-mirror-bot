# Mattermost-Slack Mirroring Bot

Mirrors chats between Mattermost and Slack, both ways. Written in golang.

## Usage

- Create a user for the bot on the Mattermost server, add it to the relevant team.
- Create a bot user for Slack.
- Run the bot:

```
$ git clone https://github.com/nilenso/mattermost-slack-mirror-bot.git
$ cd mattermost-slack-mirror-bot
$ glide install
$ go build
$ cp config.yaml.example config.yaml
$ # edit the values in config.yaml
$ ./mattermost-slack-mirror-bot config.yaml
```

- Channels are matched by their names.
- Users are matched by their emails.
- Slack messages posted by the bot appear to be posted by the matching user.
  But Mattermost messages posted by the bot bear the bot's name.
- The bot joins all public channels on the Mattermost side automatically but on the Slack side
  you'll need to invite the bot manually to each channel you want to mirror.
- If you invite the bot to a public channel on Slack, it will automatically create and/or join
  the channel with the same name on Mattermost. The reverse does not happen because bots can't
  create or join channels on Slack on their own.
- For private groups/channels, you'll need to invite the bot on both the sides to start mirroring.
