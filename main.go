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
)

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

	args := BotParams{
		MMHeartbeatInternalSeconds: 2,
	}
	if err := yaml.Unmarshal(content, &args); err != nil {
		fmt.Printf("Error in parsing config file: %+v\n", err)
		os.Exit(1)
	}

	bot, err := NewBot(&args)
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
