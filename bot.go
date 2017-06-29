package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type Bot struct {
	*Logger
	params   *BotParams
	quitChan chan struct{}
	doneChan chan struct{}

	mm    *MM
	slack *Slack
}

type BotParams struct {
	MMServerHost               string `yaml:"mm_server_host"`
	MMTeam                     string `yaml:"mm_team"`
	MMBotUserEmail             string `yaml:"mm_bot_user_email"`
	MMBotUserPassword          string `yaml:"mm_bot_user_password"`
	MMDebugChannel             string `yaml:"mm_debug_channel"`
	MMHeartbeatInternalSeconds int    `yaml:"mm_heartbeat_interval_secs"`
	SlackToken                 string `yaml:"slack_token"`
	TimezoneLocation           string `yaml:"timezone_location"`
}

func NewBot(params *BotParams) (*Bot, error) {
	bot := &Bot{
		params: params,
		mm: NewMMBot(params.MMServerHost, params.MMTeam, params.MMBotUserEmail, params.MMBotUserPassword,
			time.Duration(params.MMHeartbeatInternalSeconds)*time.Second),
		slack:    NewSlackBot(params.SlackToken),
		quitChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
	bot.slack.MM = bot.mm
	bot.mm.Slack = bot.slack

	loc, err := time.LoadLocation(params.TimezoneLocation)
	if err != nil {
		return nil, fmt.Errorf("Error in loading tz: %+v", err)
	}

	logger := bot.makeLogger(loc)
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

type ChanWriterWrapper struct {
	w io.Writer
	c chan string
}

func (wrapper *ChanWriterWrapper) Write(p []byte) (int, error) {
	n, err := wrapper.w.Write(p)
	wrapper.c <- string(p)
	return n, err
}

func (bot *Bot) makeLogger(loc *time.Location) *Logger {
	debugChan := bot.params.MMDebugChannel
	if debugChan == "" {
		return NewLogger(loc, os.Stdout)
	}

	logChan := make(chan string, 100)
	logger := NewLogger(loc, &ChanWriterWrapper{w: os.Stdout, c: logChan})

	go func() {
		for {
			select {
			case <-bot.quitChan:
				bot.doneChan <- struct{}{}
				return
			case log := <-logChan:
				if strings.Index(log, LogLevels[Debug]) == -1 {
					if err := bot.mm.PostNoLogging(debugChan, "", log); err != nil {
						fmt.Printf("Error in sending logs to MM: %+v\n", err)
					}
				}
			}
		}
	}()

	return logger
}

func (bot *Bot) Start() {
	go bot.mm.Listen()
	go bot.slack.Listen()
	select {}
}

func (bot *Bot) Stop() {
	bot.mm.Stop()
	bot.slack.Stop()
	bot.quitChan <- struct{}{}
	<-bot.doneChan
	bot.info("Stopped Bot")
}
