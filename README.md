# Mattermost-Slack Mirroring Bot

Mirrors chats between mattermost and slack, both ways. Written in golang.

## Usage
```
$ git clone https://github.com/nilenso/mattermost-slack-mirror-bot.git
$ cd mattermost-slack-mirror-bot
$ glide install
$ go build
$ ./mattermost-slack-mirror-bot <mm_server_host> <mm_team> <mm_bot_user_email> <mm_bot_user_password> <slack_token> <timezone_location>
```

`timezone_location` should be a value from the IANA Time Zone database.