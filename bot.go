package main

import (
	"fmt"
	"github.com/mattermost/platform/model"
	"github.com/nlopes/slack"
	"time"
)

type Bot struct {
	*model.User
	Server   string
	Team     string
	location *time.Location

	mmUsers       map[string]*model.User
	slackUsers    map[string]*slack.User
	slackChannels map[string]*slack.Channel

	mmClient    *model.Client
	mmWSClient  *model.WebSocketClient
	slackClient *slack.Client
}

func NewBot(server, team, email, password, slackToken, location string) (*Bot, error) {
	bot := &Bot{
		Server: server,
		Team:   team,
		User: &model.User{
			Email:    email,
			Password: password,
		},
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("Error in loading tz: %+v", err)
	}
	bot.location = loc

	if err := bot.createMMClient(); err != nil {
		return nil, fmt.Errorf("Error in creating mm client: %+v", err)
	}

	if err := bot.setMMTeam(); err != nil {
		return nil, fmt.Errorf("Error in setting up mm team: %+v", err)
	}

	if err := bot.getMMUsers(); err != nil {
		return nil, fmt.Errorf("Error in getting mm users: %+v", err)
	}

	if err := bot.joinMMChannels(); err != nil {
		return nil, fmt.Errorf("Error in joining mm channels: %+v", err)
	}

	slackClient := slack.New(slackToken)
	if _, err := slackClient.AuthTest(); err != nil {
		return nil, fmt.Errorf("Error in connecting to slack: %+v", err)
	}
	bot.slackClient = slackClient

	if err := bot.getSlackUsers(); err != nil {
		return nil, fmt.Errorf("Error in getting slack users: %+v", err)
	}

	if err := bot.getSlackChannels(); err != nil {
		return nil, fmt.Errorf("Error in getting slack channels: %+v", err)
	}

	return bot, nil
}

func (bot *Bot) createMMClient() error {
	client := model.NewClient("https://" + bot.Server)
	if _, err := client.GetPing(); err != nil {
		return err
	}
	bot.mmClient = client

	if err := bot.login(); err != nil {
		return fmt.Errorf("Error in logging in: %+v", err)
	}

	wsClient, err := model.NewWebSocketClient("wss://"+bot.Server, client.AuthToken)
	if err != nil {
		return err
	}
	bot.mmWSClient = wsClient

	return nil
}

func (bot *Bot) login() error {
	if res, err := bot.mmClient.Login(bot.Email, bot.Password); err != nil {
		return err
	} else {
		bot.User = res.Data.(*model.User)
		return nil
	}
}

func (bot *Bot) setMMTeam() error {
	if res, err := bot.mmClient.GetInitialLoad(); err != nil {
		return err
	} else {
		initialLoad := res.Data.(*model.InitialLoad)
		var botTeam *model.Team
		for _, team := range initialLoad.Teams {
			if team.Name == bot.Team {
				botTeam = team
				break
			}
		}

		if botTeam == nil {
			return fmt.Errorf("Could not find bot team: " + bot.Team)
		}

		bot.mmClient.SetTeamId(botTeam.Id)
		return nil
	}
}

func (bot *Bot) joinMMChannels() error {
	var channelList *model.ChannelList

	for {
		if channelsResult, err := bot.mmClient.GetMoreChannelsPage(0, 200); err != nil {
			return err
		} else {
			channelList = channelsResult.Data.(*model.ChannelList)
			if len(*channelList) == 0 {
				break
			}

			for _, channel := range *channelList {
				if _, err := bot.mmClient.JoinChannel(channel.Id); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (bot *Bot) GetMMUser(userId string) (*model.User, error) {
	if user, ok := bot.mmUsers[userId]; ok {
		return user, nil
	}

	if res, err := bot.mmClient.GetUser(userId, ""); err != nil {
		return nil, err
	} else {
		bot.mmUsers[userId] = res.Data.(*model.User)
		return bot.mmUsers[userId], nil
	}
}

func (bot *Bot) getMMUsers() error {
	if res, err := bot.mmClient.GetRecentlyActiveUsers(bot.mmClient.TeamId); err != nil {
		return err
	} else {
		bot.mmUsers = res.Data.(map[string]*model.User)
		return nil
	}
}

func (bot *Bot) ListenMM() chan *model.WebSocketEvent {
	bot.mmWSClient.Listen()
	return bot.mmWSClient.EventChannel
}

func (bot *Bot) Stop() {
	bot.mmWSClient.Close()
}

func (bot *Bot) getSlackUsers() error {
	if users, err := bot.slackClient.GetUsers(); err != nil {
		return err
	} else {
		userMap := make(map[string]*slack.User)
		for _, user := range users {
			email := user.Profile.Email
			if email != "" {
				user := slack.User(user)
				userMap[email] = &user
			}
		}

		bot.slackUsers = userMap
		return nil
	}
}

func (bot *Bot) getSlackChannels() error {
	if channels, err := bot.slackClient.GetChannels(true); err != nil {
		return err
	} else {
		channelMap := make(map[string]*slack.Channel)
		for _, channel := range channels {
			channel := slack.Channel(channel)
			channelMap[channel.Name] = &channel
		}

		bot.slackChannels = channelMap
		return nil
	}
}

func (bot *Bot) PostToSlack(channelName, userEmail, message string) error {
	now := time.Now().In(bot.location).Format("2006-01-02 15:04:05")

	channel, ok := bot.slackChannels[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	user, ok := bot.slackUsers[userEmail]
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

	fmt.Printf("[%s][%s][%s]: %s\n", now, channelName, userEmail, message)
	return nil
}
