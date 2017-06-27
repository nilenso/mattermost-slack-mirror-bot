package main

import (
	"errors"
	"fmt"
	mm "github.com/mattermost/platform/model"
	"time"
)

type MM struct {
	user         *mm.User
	server       string
	team         string
	users        map[string]*mm.User
	channels     map[string]*mm.Channel
	client       *mm.Client
	wsClient     *mm.WebSocketClient
	eventHandler func(*mm.WebSocketEvent)
}

var errQuit = errors.New("QUIT")

func (bot *Bot) createMMClient() error {
	client := mm.NewClient("https://" + bot.MM.server)
	if _, err := client.GetPing(); err != nil {
		return err
	}
	bot.MM.client = client

	if err := bot.login(); err != nil {
		return fmt.Errorf("Error in logging in: %+v", err)
	}

	wsClient, err := mm.NewWebSocketClient("wss://"+bot.MM.server, client.AuthToken)
	if err != nil {
		return err
	}
	bot.MM.wsClient = wsClient

	return nil
}

func (bot *Bot) login() error {
	if res, err := bot.MM.client.Login(bot.MM.user.Email, bot.MM.user.Password); err != nil {
		return err
	} else {
		bot.MM.user = res.Data.(*mm.User)
		return nil
	}
}

func (bot *Bot) setMMTeam() error {
	if res, err := bot.MM.client.GetInitialLoad(); err != nil {
		return err
	} else {
		initialLoad := res.Data.(*mm.InitialLoad)
		var botTeam *mm.Team
		for _, team := range initialLoad.Teams {
			if team.Name == bot.MM.team {
				botTeam = team
				break
			}
		}

		if botTeam == nil {
			return fmt.Errorf("Could not find bot team: " + bot.MM.team)
		}

		bot.MM.client.SetTeamId(botTeam.Id)
		return nil
	}
}

func (bot *Bot) joinMMChannels() error {
	var channelList *mm.ChannelList

	for {
		if channelsResult, err := bot.MM.client.GetMoreChannelsPage(0, 200); err != nil {
			return err
		} else {
			channelList = channelsResult.Data.(*mm.ChannelList)
			if len(*channelList) == 0 {
				break
			}

			for _, channel := range *channelList {
				if _, err := bot.MM.client.JoinChannel(channel.Id); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (bot *Bot) getMMChannels() error {
	if res, err := bot.MM.client.GetChannels(""); err != nil {
		return err
	} else {
		channelList := res.Data.(*mm.ChannelList)

		channelMap := make(map[string]*mm.Channel)
		for _, channel := range *channelList {
			c := mm.Channel(*channel)
			channelMap[channel.Name] = &c
		}
		bot.MM.channels = channelMap

		return nil
	}
}

func (bot *Bot) GetMMUser(userId string) (*mm.User, error) {
	if user, ok := bot.MM.users[userId]; ok {
		return user, nil
	}

	if res, err := bot.MM.client.GetUser(userId, ""); err != nil {
		return nil, err
	} else {
		bot.MM.users[userId] = res.Data.(*mm.User)
		return bot.MM.users[userId], nil
	}
}

func (bot *Bot) getMMUsers() error {
	if res, err := bot.MM.client.GetRecentlyActiveUsers(bot.MM.client.TeamId); err != nil {
		return err
	} else {
		bot.MM.users = res.Data.(map[string]*mm.User)
		return nil
	}
}

func (bot *Bot) ListenMM(eventHandler func(*mm.WebSocketEvent)) {
	bot.MM.eventHandler = eventHandler
	for {
		err := bot.listenMM()
		if err != nil {
			if err == errQuit {
				bot.doneChan <- struct{}{}
				return
			}
			bot.log("Error in listening to MM: %+v", err)
		}
		time.Sleep(time.Second)
	}
}

func (bot *Bot) listenMM() error {
	bot.log("Listening to MM events")

	bot.closeMMWSClient()
	if err := bot.MM.wsClient.Connect(); err != nil {
		return err
	}

	bot.MM.wsClient.Listen()
	timeoutChan := make(chan struct{})
	quitChan := make(chan struct{})
	go bot.startHeartbeat(timeoutChan, quitChan)
	for {
		select {
		case ev := <-bot.MM.wsClient.EventChannel:
			bot.MM.eventHandler(ev)
		case <-timeoutChan:
			return ErrTimeout
		case q := <-bot.quitChan:
			quitChan <- q
			bot.log("Stopped listening to MM events")
			return errQuit
		}
	}

	return nil
}

func (bot *Bot) startHeartbeat(timeoutChan chan struct{}, quitChan chan struct{}) {
	bot.log("Starting MM heartbeat")
	for {
		bot.MM.wsClient.GetStatusesByIds([]string{bot.MM.user.Id})
		timeout := time.After(bot.heartbeatInterval)
		select {
		case <-quitChan:
			bot.log("Stopped MM heartbeat")
			return
		case <-bot.MM.wsClient.ResponseChannel:
			time.Sleep(bot.heartbeatInterval)
		case <-timeout:
			timeoutChan <- struct{}{}
			return
		}
	}
}

func (bot *Bot) PostToMM(channelName, userName, message string) error {
	channel, ok := bot.MM.channels[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	_, err := bot.MM.client.CreatePost(&mm.Post{
		ChannelId: channel.Id,
		Message:   fmt.Sprintf("[@%s]: %s", userName, message),
	})

	if err != nil {
		return err
	}

	bot.log("[SK][%s][%s]: %s", channelName, userName, message)
	return nil
}

func (bot *Bot) closeMMWSClient() {
	if bot.MM.wsClient.Conn != nil {
		bot.MM.wsClient.Close()
	}
}
