package main

import (
	"errors"
	"fmt"
	mm "github.com/mattermost/platform/model"
	"github.com/nlopes/slack"
	"io"
	"log"
	"time"
)

var ErrTimeout = errors.New("Timeout")

type Bot struct {
	*MM
	*Slack

	location          *time.Location
	heartbeatInterval time.Duration
	quitChan          chan struct{}
	doneChan          chan struct{}
	logger            *log.Logger
}

func NewBot(server, team, email, password, slackToken, location string, heartbeatInterval time.Duration, logWriter io.Writer) (*Bot, error) {
	bot := &Bot{
		MM: &MM{
			server: server,
			team:   team,
			user: &mm.User{
				Email:    email,
				Password: password,
			},
		},
		Slack: &Slack{
			token: slackToken,
		},
		heartbeatInterval: heartbeatInterval,
		quitChan:          make(chan struct{}),
		doneChan:          make(chan struct{}),
		logger:            log.New(logWriter, "", 0),
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("Error in loading tz: %+v", err)
	}
	bot.location = loc

	if err := bot.createMMClient(); err != nil {
		return nil, fmt.Errorf("Error in creating mm client: %+v", err)
	}
	bot.log("Created MM client")

	if err := bot.setMMTeam(); err != nil {
		return nil, fmt.Errorf("Error in setting up mm team: %+v", err)
	}
	bot.log("Set up MM team")

	if err := bot.getMMUsers(); err != nil {
		return nil, fmt.Errorf("Error in getting mm users: %+v", err)
	}
	bot.log("Got MM users")

	if err := bot.joinMMChannels(); err != nil {
		return nil, fmt.Errorf("Error in joining mm channels: %+v", err)
	}
	bot.log("Joined MM channels")

	if err := bot.getMMChannels(); err != nil {
		return nil, fmt.Errorf("Error in getting mm channels: %+v", err)
	}
	bot.log("Got MM channels")

	if err := bot.createSlackClient(); err != nil {
		return nil, fmt.Errorf("Error in connecting to slack: %+v", err)
	}
	bot.log("Created Slack client")

	if err := bot.getSlackUsers(); err != nil {
		return nil, fmt.Errorf("Error in getting slack users: %+v", err)
	}
	bot.log("Got Slack users")

	if err := bot.getSlackChannels(); err != nil {
		return nil, fmt.Errorf("Error in getting slack channels: %+v", err)
	}
	bot.log("Got Slack channels")

	return bot, nil
}

func (bot *Bot) log(format string, args ...interface{}) {
	now := time.Now().In(bot.location).Format("2006-01-02 15:04:05")
	format = fmt.Sprintf("[%s] %s\n", now, format)
	bot.logger.Printf(format, args...)
}

func (bot *Bot) Start(mmEventHandler func(*mm.WebSocketEvent), slackEventHandler func(event *slack.RTMEvent)) {
	go bot.ListenMM(mmEventHandler)
	go bot.ListenSlack(slackEventHandler)
}

func (bot *Bot) Stop() {
	bot.closeMMWSClient()
	bot.Slack.rtmClient.Disconnect()
	bot.quitChan <- struct{}{}
	bot.quitChan <- struct{}{}
	<-bot.doneChan
	<-bot.doneChan
	bot.log("Stopped Bot")
}
