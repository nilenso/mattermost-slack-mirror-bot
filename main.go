package main

import (
	"fmt"
	mm "github.com/mattermost/platform/model"
	"github.com/nlopes/slack"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 5 {
		println("Must specify server, team, email and password for the bot")
		os.Exit(1)
	}

	server := os.Args[1]
	team := os.Args[2]
	email := os.Args[3]
	password := os.Args[4]
	slackToken := os.Args[5]
	location := os.Args[6]

	bot, err := NewBot(server, team, email, password, slackToken, location, 2*time.Second, os.Stdout)
	if err != nil {
		fmt.Printf("Error in creating bot: %+v\n", err)
		os.Exit(1)
	}

	setupGracefulShutdown(bot)
	go bot.Start(
		func(ev *mm.WebSocketEvent) { handleMMEvent(bot, ev) },
		func(ev *slack.RTMEvent) { handleSlackEvent(bot, ev) })

	go dumpThreadStacks()
	select {}
}

func dumpThreadStacks() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGQUIT)
	buf := make([]byte, 1<<20)
	for {
		<-sigs
		stacklen := runtime.Stack(buf, true)
		log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
	}
}

func handleMMEvent(bot *Bot, ev *mm.WebSocketEvent) {
	switch ev.Event {
	case mm.WEBSOCKET_EVENT_POSTED:
		handleMMPostEvent(bot, ev)
	}
}

func handleMMPostEvent(bot *Bot, event *mm.WebSocketEvent) {
	post := mm.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil && post.UserId != bot.MM.user.Id {
		user, err := bot.GetMMUser(post.UserId)
		if err != nil {
			bot.log("Error in getting MM user: %s %+v", post.UserId, err)
			return
		}

		channel := event.Data["channel_name"].(string)
		if err := bot.PostToSlack(channel, user.Email, post.Message); err != nil {
			bot.log("Error in posting to slack: %+v", err)
			return
		}
	}
}

func handleSlackEvent(bot *Bot, event *slack.RTMEvent) {
	switch ev := event.Data.(type) {
	case *slack.MessageEvent:
		handleSlackPostEvent(bot, ev)
	}
}

func handleSlackPostEvent(bot *Bot, event *slack.MessageEvent) {
	if event.User != "" {
		user, ok := bot.GetSlackUser(event.User)
		if !ok {
			bot.log("Error in getting Slack user: %s", event.User)
			return
		}

		channel, ok := bot.GetSlackChannel(event.Channel)
		if !ok {
			bot.log("Error in getting Slack channel: %s", event.Channel)
			return
		}

		text := bot.SubsSlackUserIdMentions(event.Text)
		if err := bot.PostToMM(channel.Name, user.Name, text); err != nil {
			bot.log("Error in posting to MM: %+v", err)
			return
		}
	}
}

func setupGracefulShutdown(bot *Bot) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			bot.Stop()
			os.Exit(0)
		}
	}()
}
