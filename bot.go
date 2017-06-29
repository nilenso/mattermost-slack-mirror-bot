package main

import (
	"fmt"
	"os"
	"time"
)

type Bot struct {
	*Logger
	mm    *MM
	slack *Slack
}

func NewBot(server, team, email, password, slackToken, location string, heartbeatInterval time.Duration) (*Bot, error) {
	bot := &Bot{
		mm:    NewMMBot(server, team, email, password, heartbeatInterval),
		slack: NewSlackBot(slackToken),
	}
	bot.slack.MM = bot.mm
	bot.mm.Slack = bot.slack

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, fmt.Errorf("Error in loading tz: %+v", err)
	}
	logger := NewLogger(loc, os.Stdout)
	bot.Logger = logger
	bot.mm.Logger = logger
	bot.slack.Logger = logger

	if err := bot.mm.Start(); err != nil {
		return nil, err
	}

	if err := bot.slack.Start(); err != nil {
		return nil, err
	}

	return bot, nil
}

func (bot *Bot) Start() {
	go bot.mm.Listen()
	go bot.slack.Listen()
	select {}
}

func (bot *Bot) Stop() {
	bot.mm.Stop()
	bot.slack.Stop()
	bot.info("Stopped Bot")
}
