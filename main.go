package main

import (
	"fmt"
	"github.com/mattermost/platform/model"
	"os"
	"os/signal"
	"strings"
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

	bot, err := NewBot(server, team, email, password, slackToken, location)
	if err != nil {
		fmt.Printf("Error in creating bot, :%v\n", err)
		os.Exit(1)
	}
	setupGracefulShutdown(bot)
	defer bot.Stop()
	fmt.Println("Connected the bot")

	eventChan := bot.ListenMM()
	fmt.Println("Listening to mm events")
	go func() {
		for {
			select {
			case ev := <-eventChan:
				if ev.Event == model.WEBSOCKET_EVENT_POSTED {
					handlePostEvent(bot, ev)
				}
			}
		}
	}()

	select {}
}
func handlePostEvent(bot *Bot, event *model.WebSocketEvent) {
	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {
		user, err := bot.GetMMUser(post.UserId)
		if err != nil {
			fmt.Printf("Error in getting mm user: %s %+v\n", post.UserId, err)
			return
		}

		channel := event.Data["channel_name"].(string)
		if err := bot.PostToSlack(channel, user.Email, post.Message); err != nil {
			fmt.Printf("Error in posting to slack: %+v", err)
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
			fmt.Println("Stopped bot")
			os.Exit(0)
		}
	}()
}
