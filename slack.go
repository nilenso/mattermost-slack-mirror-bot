package main

import (
	"fmt"
	"github.com/nlopes/slack"
	"regexp"
	"strings"
)

type Slack struct {
	*Logger

	quitChan chan struct{}
	doneChan chan struct{}

	token          string
	usersByEmail   map[string]*slack.User
	channelsByName map[string]*slack.Channel
	usersByID      map[string]*slack.User
	channelsByID   map[string]*slack.Channel
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
	serverInfo, err := bot.createClient()
	if err != nil {
		return fmt.Errorf("Error in connecting to slack: %+v", err)
	}
	bot.info("Created Slack client")

	if err := bot.getUsers(); err != nil {
		return fmt.Errorf("Error in getting slack users: %+v", err)
	}
	bot.info("Got Slack users")

	if err := bot.getChannels(); err != nil {
		return fmt.Errorf("Error in getting slack channels: %+v", err)
	}
	bot.info("Got Slack channels")
	bot.info("Connected to %s Slack", serverInfo.Team)

	return nil
}

func (bot *Slack) Stop() {
	_ = bot.rtmClient.Disconnect()
	bot.quitChan <- struct{}{}
	<-bot.doneChan
}

func (bot *Slack) createClient() (*slack.AuthTestResponse, error) {
	slackClient := slack.New(bot.token)

	serverInfo, err := slackClient.AuthTest()
	if err != nil {
		return nil, err
	}
	bot.client = slackClient

	bot.rtmClient = bot.client.NewRTM()
	go bot.rtmClient.ManageConnection()

	return serverInfo, nil
}

func (bot *Slack) getUsers() error {
	if users, err := bot.client.GetUsers(); err != nil {
		return err
	} else {
		userEmailMap := make(map[string]*slack.User)
		userIDMap := make(map[string]*slack.User)

		for _, user := range users {
			email := user.Profile.Email
			if email != "" {
				user := slack.User(user)
				userEmailMap[email] = &user
				userIDMap[user.ID] = &user
			}
		}

		bot.usersByEmail = userEmailMap
		bot.usersByID = userIDMap
		return nil
	}
}

func (bot *Slack) GetUser(userID string) (*slack.User, bool) {
	user, ok := bot.usersByID[userID]
	return user, ok
}

func (bot *Slack) getChannels() error {
	if channels, err := bot.client.GetChannels(true); err != nil {
		return err
	} else {
		channelNameMap := make(map[string]*slack.Channel)
		channelIDMap := make(map[string]*slack.Channel)
		for _, channel := range channels {
			channel := slack.Channel(channel)
			channelNameMap[channel.Name] = &channel
			channelIDMap[channel.ID] = &channel
		}

		bot.channelsByName = channelNameMap
		bot.channelsByID = channelIDMap
		return nil
	}
}

func (bot *Slack) GetChannel(channelID string) (*slack.Channel, bool) {
	channel, ok := bot.channelsByID[channelID]
	return channel, ok
}

func (bot *Slack) Listen() {
	bot.info("Listening to Slack events")

	for {
		select {
		case ev := <-bot.rtmClient.IncomingEvents:
			bot.handleEvent(&ev)
		case <-bot.quitChan:
			bot.info("Stopped listening to Slack events")
			bot.doneChan <- struct{}{}
			return
		}
	}
}

func (bot *Slack) handleEvent(event *slack.RTMEvent) {
	defer func() {
		if r := recover(); r != nil {
			bot.error("Recovered while handling Slack event: %+v", r)
		}
	}()

	switch ev := event.Data.(type) {
	case *slack.MessageEvent:
		bot.handlePostEvent(ev)
	case *slack.ChannelJoinedEvent:
		bot.handleChannelJoinEvent(ev, false)
	case *slack.GroupJoinedEvent:
		jEv := slack.ChannelJoinedEvent(*ev)
		bot.handleChannelJoinEvent(&jEv, true)
	case *slack.TeamJoinEvent:
		bot.handleTeamJoinEvent(ev)
	}
}

func (bot *Slack) handleTeamJoinEvent(event *slack.TeamJoinEvent) {
	bot.usersByEmail[event.User.Profile.Email] = &event.User
	bot.usersByID[event.User.ID] = &event.User
	bot.info("User %s joined the Slack team", event.User.Name)
}

func (bot *Slack) handleChannelJoinEvent(event *slack.ChannelJoinedEvent, private bool) {
	channel := event.Channel
	bot.channelsByID[channel.ID] = &channel
	bot.channelsByName[channel.Name] = &channel
	bot.info("Joined Slack channel: %s", channel.Name)

	if !private {
		if err := bot.MM.CreateAndJoinChannel(channel.Name); err != nil {
			bot.error("Error in creating/joining MM channel: %s", channel.Name)
		}
	}
}

func (bot *Slack) handlePostEvent(event *slack.MessageEvent) {
	if event.User != "" {
		user, ok := bot.GetUser(event.User)
		if !ok {
			bot.error("Error in getting Slack user: %s", event.User)
			return
		}

		channel, ok := bot.GetChannel(event.Channel)
		if !ok {
			bot.error("Error in getting Slack channel: %s", event.Channel)
			return
		}

		text := bot.subsUserIDMentions(event.Text)
		if err := bot.MM.Post(channel.Name, user.Name, text); err != nil {
			bot.error("Error in posting to MM: %+v", err)
			return
		}
	}
}

var userIDMentionRe = regexp.MustCompile(`<@U[A-Z0-9]+>`)

func (bot *Slack) subsUserIDMentions(text string) string {
	res := userIDMentionRe.FindAllStringSubmatch(text, -1)
	if len(res) == 0 {
		return text
	}

	subsText := text
	for _, match := range res {
		userMention := match[0]
		userID := strings.TrimSuffix(strings.TrimPrefix(userMention, "<@"), ">")
		user, ok := bot.GetUser(userID)
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

	bot.debug("[SK][%s][%s]: %s", channelName, userEmail, message)
	return nil
}
