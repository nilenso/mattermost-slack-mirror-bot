package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type ProgArgs struct {
	MMServerHost      string `yaml:"mm_server_host"`
	MMTeam            string `yaml:"mm_team"`
	MMBotUserEmail    string `yaml:"mm_bot_user_email"`
	MMBotUserPassword string `yaml:"mm_bot_user_password"`
	SlackToken        string `yaml:"slack_token"`
	TimezoneLocation  string `yaml:"timezone_location"`
}

func main() {
	if len(os.Args) < 2 {
		println("Usage: ./mattermost-slack-mirror-bot <config_file>")
		os.Exit(1)
	}

	configFile := os.Args[1]
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Could not read config file: %s\n", configFile)
		os.Exit(1)
	}

	args := ProgArgs{}
	if err := yaml.Unmarshal(content, &args); err != nil {
		fmt.Printf("Error in parsing config file: %+v\n", err)
		os.Exit(1)
	}

	bot, err := NewBot(
		args.MMServerHost, args.MMTeam, args.MMBotUserEmail, args.MMBotUserPassword,
		args.SlackToken, args.TimezoneLocation, 2*time.Second)
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
