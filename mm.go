package main

import (
	"errors"
	"fmt"
	mm "github.com/mattermost/platform/model"
	"strings"
	"time"
)

var (
	ErrTimeout    = errors.New("Timeout")
	ErrChanClosed = errors.New("Listen channel closed")
)

type MM struct {
	*Logger

	heartbeatInterval time.Duration
	quitChan          chan struct{}
	doneChan          chan struct{}

	user     *mm.User
	server   string
	team     string
	users    map[string]*mm.User
	channels map[string]*mm.Channel
	client   *mm.Client
	wsClient *mm.WebSocketClient

	Slack *Slack
}

var errQuit = errors.New("QUIT")

func NewMMBot(server, team, email, password string, heartbeatInterval time.Duration) *MM {
	return &MM{
		server: server,
		team:   team,
		user: &mm.User{
			Email:    email,
			Password: password,
		},
		heartbeatInterval: heartbeatInterval,
		quitChan:          make(chan struct{}),
		doneChan:          make(chan struct{}),
	}
}

func (bot *MM) Start() error {
	if err := bot.createClient(); err != nil {
		return fmt.Errorf("Error in creating mm client: %+v", err)
	}
	bot.info("Created MM client")

	if err := bot.setTeam(); err != nil {
		return fmt.Errorf("Error in setting up mm team: %+v", err)
	}
	bot.info("Set up MM team")

	if err := bot.getUsers(); err != nil {
		return fmt.Errorf("Error in getting mm users: %+v", err)
	}
	bot.info("Got MM users")

	if err := bot.joinChannels(); err != nil {
		return fmt.Errorf("Error in joining mm channels: %+v", err)
	}
	bot.info("Joined MM channels")

	if err := bot.getChannels(); err != nil {
		return fmt.Errorf("Error in getting mm channels: %+v", err)
	}
	bot.info("Got MM channels")
	bot.info("Connected to %s/%s", bot.server, bot.team)

	return nil
}

func (bot *MM) Stop() {
	bot.closeWSClient()
	bot.quitChan <- struct{}{}
	<-bot.doneChan
}

func (bot *MM) createClient() error {
	client := mm.NewClient("https://" + bot.server)
	if _, err := client.GetPing(); err != nil {
		return err
	}
	bot.client = client

	if err := bot.login(); err != nil {
		return fmt.Errorf("Error in logging in: %+v", err)
	}

	wsClient, err := mm.NewWebSocketClient("wss://"+bot.server, client.AuthToken)
	if err != nil {
		return err
	}
	bot.wsClient = wsClient

	return nil
}

func (bot *MM) login() error {
	if res, err := bot.client.Login(bot.user.Email, bot.user.Password); err != nil {
		return err
	} else {
		bot.user = res.Data.(*mm.User)
		return nil
	}
}

func (bot *MM) setTeam() error {
	if res, err := bot.client.GetInitialLoad(); err != nil {
		return err
	} else {
		initialLoad := res.Data.(*mm.InitialLoad)
		var botTeam *mm.Team
		for _, team := range initialLoad.Teams {
			if team.Name == bot.team {
				botTeam = team
				break
			}
		}

		if botTeam == nil {
			return fmt.Errorf("Could not find bot team: " + bot.team)
		}

		bot.client.SetTeamId(botTeam.Id)
		return nil
	}
}

func (bot *MM) joinChannels() error {
	var channelList *mm.ChannelList

	for {
		if channelsResult, err := bot.client.GetMoreChannelsPage(0, 200); err != nil {
			return err
		} else {
			channelList = channelsResult.Data.(*mm.ChannelList)
			if len(*channelList) == 0 {
				break
			}

			for _, channel := range *channelList {
				if _, err := bot.client.JoinChannel(channel.Id); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (bot *MM) getChannels() error {
	if res, err := bot.client.GetChannels(""); err != nil {
		return err
	} else {
		channelList := res.Data.(*mm.ChannelList)

		channelMap := make(map[string]*mm.Channel)
		for _, channel := range *channelList {
			c := mm.Channel(*channel)
			channelMap[channel.Name] = &c
		}
		bot.channels = channelMap

		return nil
	}
}

func (bot *MM) GetUser(userID string) (*mm.User, error) {
	if user, ok := bot.users[userID]; ok {
		return user, nil
	}

	if res, err := bot.client.GetUser(userID, ""); err != nil {
		return nil, err
	} else {
		bot.users[userID] = res.Data.(*mm.User)
		return bot.users[userID], nil
	}
}

func (bot *MM) getUsers() error {
	if res, err := bot.client.GetRecentlyActiveUsers(bot.client.TeamId); err != nil {
		return err
	} else {
		bot.users = res.Data.(map[string]*mm.User)
		return nil
	}
}

func (bot *MM) CreateAndJoinChannel(channelName string) error {
	if _, ok := bot.channels[channelName]; ok {
		return nil
	}

	if res, err := bot.client.CreateChannel(&mm.Channel{
		Name:        channelName,
		DisplayName: channelName,
		Type:        "O",
	}); err != nil {
		return err
	} else {
		channel := res.Data.(*mm.Channel)
		bot.info("Created MM channel: %s", channel.Name)

		if _, err := bot.client.JoinChannel(channel.Id); err != nil {
			return err
		}
		bot.channels[channel.Name] = channel
		bot.info("Joined MM channel: %s", channel.Name)
	}

	return nil
}

func (bot *MM) Listen() {
	for {
		err := bot.listen()
		if err != nil {
			if err == errQuit {
				bot.doneChan <- struct{}{}
				return
			}
			bot.error("Error in listening to MM: %+v", err)
		}
		time.Sleep(time.Second)
	}
}

func (bot *MM) listen() error {
	bot.info("Listening to MM events")

	bot.closeWSClient()
	if err := bot.wsClient.Connect(); err != nil {
		return err
	}

	bot.wsClient.Listen()
	timeoutChan := make(chan struct{})
	quitChan := make(chan struct{})
	go bot.startHeartbeat(timeoutChan, quitChan)
	for {
		select {
		case ev, ok := <-bot.wsClient.EventChannel:
			if !ok {
				return ErrChanClosed
			}
			bot.handleEvent(ev)
		case <-timeoutChan:
			return ErrTimeout
		case q := <-bot.quitChan:
			quitChan <- q
			bot.info("Stopped listening to MM events")
			return errQuit
		}
	}

	return nil
}

func (bot *MM) startHeartbeat(timeoutChan chan struct{}, quitChan chan struct{}) {
	bot.info("Starting MM heartbeat")
	for {
		bot.wsClient.GetStatusesByIds([]string{bot.user.Id})
		timeout := time.After(bot.heartbeatInterval)
		select {
		case <-quitChan:
			bot.info("Stopped MM heartbeat")
			return
		case <-bot.wsClient.ResponseChannel:
			time.Sleep(bot.heartbeatInterval)
		case <-timeout:
			timeoutChan <- struct{}{}
			return
		}
	}
}

func (bot *MM) handleEvent(ev *mm.WebSocketEvent) {
	defer func() {
		if r := recover(); r != nil {
			bot.error("Recovered while handling MM event: %+v %+v", ev, r)
		}
	}()

	switch ev.Event {
	case mm.WEBSOCKET_EVENT_POSTED:
		bot.handlePostEvent(ev)
	case mm.WEBSOCKET_EVENT_ADDED_TO_TEAM:
		bot.handleTeamJoinEvent(ev)
	}
}
func (bot *MM) handleTeamJoinEvent(event *mm.WebSocketEvent) {
	userID := event.Data["user_id"].(string)
	if res, err := bot.client.GetUser(userID, ""); err != nil {
		bot.error("Error in getting MM user: %s %+v", userID, err)
	} else {
		user := res.Data.(*mm.User)
		bot.users[user.Id] = user
		bot.info("User %s joined the MM team", user.Username)
	}
}

func (bot *MM) handlePostEvent(event *mm.WebSocketEvent) {
	post := mm.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil && post.UserId != bot.user.Id {
		channelName := event.Data["channel_name"].(string)
		switch post.Type {
		case mm.POST_ADD_TO_CHANNEL:
			bot.handleChannelJoinEvent(post, channelName)
		default:
			bot.handleMessagePostEvent(post, channelName)
		}
	}
}

func (bot *MM) handleChannelJoinEvent(post *mm.Post, channelName string) {
	if res, err := bot.client.GetChannel(post.ChannelId, ""); err != nil {
		bot.error("Error in getting MM channel: %s %+v", channelName, err)
		return
	} else {
		channel := res.Data.(*mm.ChannelData).Channel
		bot.channels[channel.Name] = channel
		bot.info("Joined MM channel: %s", channel.Name)
	}
}

func (bot *MM) handleMessagePostEvent(post *mm.Post, channelName string) {
	if strings.Index(post.Type, mm.POST_SYSTEM_MESSAGE_PREFIX) == 0 {
		// system event. nothing to do.
		return
	}

	user, err := bot.GetUser(post.UserId)
	if err != nil {
		bot.error("Error in getting MM user: %s %+v", post.UserId, err)
		return
	}

	if err := bot.Slack.Post(channelName, user.Email, post.Message); err != nil {
		bot.error("Error in posting to slack: %+v", err)
		return
	}
}

func (bot *MM) Post(channelName, userName, message string) error {
	return bot.post(channelName, userName, message, true)
}

func (bot *MM) PostNoLogging(channelName, userName, message string) error {
	return bot.post(channelName, userName, message, false)
}

func (bot *MM) post(channelName, userName, message string, logDebug bool) error {
	channel, ok := bot.channels[channelName]
	if !ok {
		return fmt.Errorf("Could not find channel: %s", channelName)
	}

	msg := message
	if userName != "" {
		msg = fmt.Sprintf("<@%s>: %s", userName, message)
	}
	_, err := bot.client.CreatePost(&mm.Post{ChannelId: channel.Id, Message: msg})

	if err != nil {
		return err
	}

	if logDebug {
		bot.debug("[MM][%s][%s]: %s", channelName, userName, message)
	}
	return nil
}

func (bot *MM) closeWSClient() {
	if bot.wsClient.Conn != nil {
		bot.wsClient.Close()
	}
}
