package main

import (
	"fmt"
	"github.com/nlopes/slack"
)

func (bot *Bot) createSlackClient() error {
	slackClient := slack.New(bot.slackToken)
	if _, err := slackClient.AuthTest(); err != nil {
		return err
	}
	bot.slackClient = slackClient

	bot.slackRTMClient = bot.slackClient.NewRTM()
	go bot.slackRTMClient.ManageConnection()

	return nil
}

func (bot *Bot) getSlackUsers() error {
	if users, err := bot.slackClient.GetUsers(); err != nil {
		return err
	} else {
		userEmailMap := make(map[string]*slack.User)
		userIdMap := make(map[string]*slack.User)

		for _, user := range users {
			email := user.Profile.Email
			if email != "" {
				user := slack.User(user)
				userEmailMap[email] = &user
				userIdMap[user.ID] = &user
			}
		}

		bot.slackUsersByEmail = userEmailMap
		bot.slackUsersById = userIdMap
		return nil
	}
}

func (bot *Bot) GetSlackUser(userId string) (*slack.User, bool) {
	user, ok := bot.slackUsersById[userId]
	return user, ok
}

func (bot *Bot) getSlackChannels() error {
	if channels, err := bot.slackClient.GetChannels(true); err != nil {
		return err
	} else {
		channelNameMap := make(map[string]*slack.Channel)
		channelIdMap := make(map[string]*slack.Channel)
		for _, channel := range channels {
			channel := slack.Channel(channel)
			channelNameMap[channel.Name] = &channel
			channelIdMap[channel.ID] = &channel
		}

		bot.slackChannelsByName = channelNameMap
		bot.slackChannelsById = channelIdMap
		return nil
	}
}

func (bot *Bot) GetSlackChannel(channelId string) (*slack.Channel, bool) {
	channel, ok := bot.slackChannelsById[channelId]
	return channel, ok
}

func (bot *Bot) ListenSlack(eventHandler func(event *slack.RTMEvent)) {
	bot.log("Listening to Slack events")

	for {
		select {
		case msg := <-bot.slackRTMClient.IncomingEvents:
			eventHandler(&msg)
		case <-bot.quitChan:
			bot.log("Stopped listening to Slack events")
			bot.doneChan <- struct{}{}
			return
		}
	}
}

func (bot *Bot) PostToSlack(channelName, userEmail, message string) error {
	channel, ok := bot.slackChannelsByName[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	user, ok := bot.slackUsersByEmail[userEmail]
	if !ok {
		return fmt.Errorf("Could not find user: %s", userEmail)
	}
	_, _, err := bot.slackClient.PostMessage(channel.ID, message, slack.PostMessageParameters{
		Username:  user.Name,
		IconURL:   user.Profile.Image48,
		LinkNames: 1,
	})

	if err != nil {
		return err
	}

	bot.log("[MM][%s][%s]: %s", channelName, userEmail, message)
	return nil
}
