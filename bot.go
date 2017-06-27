package main

import (
	"fmt"
	mm "github.com/mattermost/platform/model"
	"github.com/nlopes/slack"
	"io"
	"log"
	"time"
)

type Bot struct {
	mm    *MM
	slack *Slack

	location *time.Location
	logger   *log.Logger
}

func NewBot(server, team, email, password, slackToken, location string, heartbeatInterval time.Duration, logWriter io.Writer) (*Bot, error) {
	bot := &Bot{
		mm:     NewMMBot(server, team, email, password, heartbeatInterval),
		slack:  NewSlackBot(slackToken),
		logger: log.New(logWriter, "", 0),
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("Error in loading tz: %+v", err)
	}
	bot.location = loc
	bot.mm.log = bot.log
	bot.slack.log = bot.log

	if err := bot.mm.Start(); err != nil {
		return nil, err
	}

	if err := bot.slack.Start(); err != nil {
		return nil, err
	}

	return bot, nil
}

func (bot *Bot) log(format string, args ...interface{}) {
	now := time.Now().In(bot.location).Format("2006-01-02 15:04:05")
	format = fmt.Sprintf("[%s] %s\n", now, format)
	bot.logger.Printf(format, args...)
}

func (bot *Bot) Start(mmEventHandler func(*mm.WebSocketEvent), slackEventHandler func(event *slack.RTMEvent)) {
	go bot.mm.Listen(mmEventHandler)
	go bot.slack.Listen(slackEventHandler)
}

func (bot *Bot) Stop() {
	bot.mm.Stop()
	bot.slack.Stop()
	bot.log("Stopped Bot")
}
