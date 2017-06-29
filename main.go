package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 7 {
		println("Usage: ./mattermost-slack-mirror-bot <mm_server_host> <mm_team> <mm_bot_user_email> <mm_bot_user_password> <slack_token> <timezone_location>")
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

	go setupGracefulShutdown(bot)
	go dumpThreadStacks()

	bot.Start()
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

func setupGracefulShutdown(bot *Bot) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for range c {
		bot.Stop()
		os.Exit(0)
	}
}
