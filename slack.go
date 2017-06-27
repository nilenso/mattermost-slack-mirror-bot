package main

import (
	"fmt"
	"github.com/nlopes/slack"
	"regexp"
	"strings"
)

type Slack struct {
	quitChan chan struct{}
	doneChan chan struct{}
	log      func(format string, args ...interface{})

	token          string
	usersByEmail   map[string]*slack.User
	channelsByName map[string]*slack.Channel
	usersById      map[string]*slack.User
	channelsById   map[string]*slack.Channel
	client         *slack.Client
	rtmClient      *slack.RTM

	MM *MM
}

func NewSlackBot(slackToken string) *Slack {
	return &Slack{
		token:    slackToken,
		quitChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

func (bot *Slack) Start() error {
	if err := bot.createClient(); err != nil {
		return fmt.Errorf("Error in connecting to slack: %+v", err)
	}
	bot.log("Created Slack client")

	if err := bot.getUsers(); err != nil {
		return fmt.Errorf("Error in getting slack users: %+v", err)
	}
	bot.log("Got Slack users")

	if err := bot.getChannels(); err != nil {
		return fmt.Errorf("Error in getting slack channels: %+v", err)
	}
	bot.log("Got Slack channels")

	return nil
}

func (bot *Slack) Stop() {
	bot.rtmClient.Disconnect()
	bot.quitChan <- struct{}{}
	<-bot.doneChan
}

func (bot *Slack) createClient() error {
	slackClient := slack.New(bot.token)
	if _, err := slackClient.AuthTest(); err != nil {
		return err
	}
	bot.client = slackClient

	bot.rtmClient = bot.client.NewRTM()
	go bot.rtmClient.ManageConnection()

	return nil
}

func (bot *Slack) getUsers() error {
	if users, err := bot.client.GetUsers(); err != nil {
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

		bot.usersByEmail = userEmailMap
		bot.usersById = userIdMap
		return nil
	}
}

func (bot *Slack) GetUser(userId string) (*slack.User, bool) {
	user, ok := bot.usersById[userId]
	return user, ok
}

func (bot *Slack) getChannels() error {
	if channels, err := bot.client.GetChannels(true); err != nil {
		return err
	} else {
		channelNameMap := make(map[string]*slack.Channel)
		channelIdMap := make(map[string]*slack.Channel)
		for _, channel := range channels {
			channel := slack.Channel(channel)
			channelNameMap[channel.Name] = &channel
			channelIdMap[channel.ID] = &channel
		}

		bot.channelsByName = channelNameMap
		bot.channelsById = channelIdMap
		return nil
	}
}

func (bot *Slack) GetChannel(channelId string) (*slack.Channel, bool) {
	channel, ok := bot.channelsById[channelId]
	return channel, ok
}

func (bot *Slack) Listen() {
	bot.log("Listening to Slack events")

	for {
		select {
		case ev := <-bot.rtmClient.IncomingEvents:
			bot.handleEvent(&ev)
		case <-bot.quitChan:
			bot.log("Stopped listening to Slack events")
			bot.doneChan <- struct{}{}
			return
		}
	}
}

func (bot *Slack) handleEvent(event *slack.RTMEvent) {
	switch ev := event.Data.(type) {
	case *slack.MessageEvent:
		bot.handlePostEvent(ev)
	case *slack.ChannelJoinedEvent:
		bot.handleChannelJoinEvent(ev, false)
	case *slack.GroupJoinedEvent:
		jEv := slack.ChannelJoinedEvent(*ev)
		bot.handleChannelJoinEvent(&jEv, true)
	}
}

func (bot *Slack) handleChannelJoinEvent(event *slack.ChannelJoinedEvent, private bool) {
	channel := event.Channel
	bot.channelsById[channel.ID] = &channel
	bot.channelsByName[channel.Name] = &channel
	bot.log("Joined Slack channel: %s", channel.Name)

	if !private {
		bot.MM.CreateAndJoinChannel(channel.Name)
	}
}

func (bot *Slack) handlePostEvent(event *slack.MessageEvent) {
	if event.User != "" {
		user, ok := bot.GetUser(event.User)
		if !ok {
			bot.log("Error in getting Slack user: %s", event.User)
			return
		}

		channel, ok := bot.GetChannel(event.Channel)
		if !ok {
			bot.log("Error in getting Slack channel: %s", event.Channel)
			return
		}

		text := bot.subsUserIdMentions(event.Text)
		if err := bot.MM.Post(channel.Name, user.Name, text); err != nil {
			bot.log("Error in posting to MM: %+v", err)
			return
		}
	}
}

var userIdMentionRe = regexp.MustCompile(`<@U[A-Z0-9]+>`)

func (bot *Slack) subsUserIdMentions(text string) string {
	res := userIdMentionRe.FindAllStringSubmatch(text, -1)
	if len(res) == 0 {
		return text
	}

	subsText := text
	for _, match := range res {
		userMention := match[0]
		userId := strings.TrimSuffix(strings.TrimPrefix(userMention, "<@"), ">")
		user, ok := bot.GetUser(userId)
		if !ok {
			continue
		}

		subsText = strings.Replace(subsText, userMention, fmt.Sprintf("@%s", user.Name), 1)
	}

	return subsText
}

func (bot *Slack) Post(channelName, userEmail, message string) error {
	channel, ok := bot.channelsByName[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	user, ok := bot.usersByEmail[userEmail]
	if !ok {
		return fmt.Errorf("Could not find user: %s", userEmail)
	}
	_, _, err := bot.client.PostMessage(channel.ID, message, slack.PostMessageParameters{
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
