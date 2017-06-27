package main

import (
	"fmt"
	"github.com/nlopes/slack"
)

type Slack struct {
	token          string
	usersByEmail   map[string]*slack.User
	channelsByName map[string]*slack.Channel
	usersById      map[string]*slack.User
	channelsById   map[string]*slack.Channel
	client         *slack.Client
	rtmClient      *slack.RTM
}

func (bot *Bot) createSlackClient() error {
	slackClient := slack.New(bot.Slack.token)
	if _, err := slackClient.AuthTest(); err != nil {
		return err
	}
	bot.Slack.client = slackClient

	bot.Slack.rtmClient = bot.Slack.client.NewRTM()
	go bot.Slack.rtmClient.ManageConnection()

	return nil
}

func (bot *Bot) getSlackUsers() error {
	if users, err := bot.Slack.client.GetUsers(); err != nil {
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

		bot.Slack.usersByEmail = userEmailMap
		bot.Slack.usersById = userIdMap
		return nil
	}
}

func (bot *Bot) GetSlackUser(userId string) (*slack.User, bool) {
	user, ok := bot.Slack.usersById[userId]
	return user, ok
}

func (bot *Bot) getSlackChannels() error {
	if channels, err := bot.Slack.client.GetChannels(true); err != nil {
		return err
	} else {
		channelNameMap := make(map[string]*slack.Channel)
		channelIdMap := make(map[string]*slack.Channel)
		for _, channel := range channels {
			channel := slack.Channel(channel)
			channelNameMap[channel.Name] = &channel
			channelIdMap[channel.ID] = &channel
		}

		bot.Slack.channelsByName = channelNameMap
		bot.Slack.channelsById = channelIdMap
		return nil
	}
}

func (bot *Bot) GetSlackChannel(channelId string) (*slack.Channel, bool) {
	channel, ok := bot.Slack.channelsById[channelId]
	return channel, ok
}

func (bot *Bot) ListenSlack(eventHandler func(event *slack.RTMEvent)) {
	bot.log("Listening to Slack events")

	for {
		select {
		case msg := <-bot.Slack.rtmClient.IncomingEvents:
			eventHandler(&msg)
		case <-bot.quitChan:
			bot.log("Stopped listening to Slack events")
			bot.doneChan <- struct{}{}
			return
		}
	}
}

func (bot *Bot) PostToSlack(channelName, userEmail, message string) error {
	channel, ok := bot.Slack.channelsByName[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	user, ok := bot.Slack.usersByEmail[userEmail]
	if !ok {
		return fmt.Errorf("Could not find user: %s", userEmail)
	}
	_, _, err := bot.Slack.client.PostMessage(channel.ID, message, slack.PostMessageParameters{
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
