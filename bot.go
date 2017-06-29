package main

import (
	"fmt"
	"os"
	"time"
)

type Bot struct {
	*Logger
	params *BotParams

	mm    *MM
	slack *Slack
}

type BotParams struct {
	MMServerHost               string `yaml:"mm_server_host"`
	MMTeam                     string `yaml:"mm_team"`
	MMBotUserEmail             string `yaml:"mm_bot_user_email"`
	MMBotUserPassword          string `yaml:"mm_bot_user_password"`
	MMHeartbeatInternalSeconds int    `yaml:"mm_heartbeat_interval_secs"`
	SlackToken                 string `yaml:"slack_token"`
	TimezoneLocation           string `yaml:"timezone_location"`
}

func NewBot(params *BotParams) (*Bot, error) {
	bot := &Bot{
		params: params,
		mm: NewMMBot(params.MMServerHost, params.MMTeam, params.MMBotUserEmail, params.MMBotUserPassword,
			time.Duration(params.MMHeartbeatInternalSeconds)*time.Second),
		slack: NewSlackBot(params.SlackToken),
	}
	bot.slack.MM = bot.mm
	bot.mm.Slack = bot.slack

	loc, err := time.LoadLocation(params.TimezoneLocation)
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
