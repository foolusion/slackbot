package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

var token = flag.String("token", "", "token for the api")

// SlackMessage is the structure of messages sent to the server.
type SlackMessage struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

// SlackError is the code and error message received when something goes wrong.
type SlackError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

// SlackEvent is the main struct for data that is received from the Real Time Messaging Connection.
type SlackEvent struct {
	OK        bool       `json:"ok"`
	Type      string     `json:"type"`
	Error     SlackError `json:"error"`
	Timestamp string     `json:"ts"`
	Text      string     `json:"text"`
	Channel   string     `json:"channel"`
	User      string     `json:"user"`
	Subtype   string     `json:"subtype"`
}

// SlackAuthenticatedUser contains information about the authenticated user.
type SlackAuthenticatedUser struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Created        int    `json:"created"`
	ManualPresence string `json:"manual_presence"`
}

// SlackTeam is the information pertaining to the team.
type SlackTeam struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	EmailDomain              string `json:"email_domain"`
	MessageEditWindowMinutes int    `json:"msg_edit_window_mins"`
	OverStorageLimit         bool   `json:"over_storage_limit"`
	Plan                     string `json:"plan"`
}

// SlackUser is the information about users/bots in the team.
type SlackUser struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Deleted           bool             `json:"deleted"`
	Color             string           `json:"color"`
	Profile           SlackUserProfile `json:"profile"`
	IsAdmin           bool             `json:"is_admin"`
	IsOwner           bool             `json:"is_owner"`
	IsPrimaryOwner    bool             `json:"is_primary_owner"`
	IsRestricted      bool             `json:"is_restricted"`
	IsUltraRestricted bool             `json:"is_ultra_restricted"`
	HasTwoFactorAuth  bool             `json:"has_2fa"`
	HasFiles          bool             `json:"has_files"`
}

// SlackUserProfile is the user profile data for users and their images.
type SlackUserProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RealName  string `json:"real_name"`
	Email     string `json:"email"`
	Skype     string `json:"skype"`
	Phone     string `json:"phone"`
	Image24   string `json:"image_24"`
	Image32   string `json:"image_32"`
	Image48   string `json:"image_48"`
	Image72   string `json:"image_72"`
	Image192  string `json:"image_192"`
}

// SlackChannel has information about the channels.
type SlackChannel struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	IsChannel          bool              `json:"is_channel"`
	Created            int               `json:"created"`
	Creator            string            `json:"creator"`
	IsArchived         bool              `json:"is_archived"`
	IsGeneral          bool              `json:"is_general"`
	Members            []string          `json:"members"`
	Topic              SlackChannelTopic `json:"topic"`
	Purpose            SlackChannelTopic `json:"purpose"`
	IsMember           bool              `json:"is_member"`
	LastRead           string            `json:"last_read"`
	Latest             json.RawMessage   `json:"latest"`
	UnreadCount        int               `json:"unread_count"`
	UnreadCountDisplay int               `json:"unread_count_display"`
}

// SlackChannelTopic is the topic or purpose of the channel.
type SlackChannelTopic struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int    `json:"last_set"`
}

type rtmStart struct {
	Ok       bool                   `json:"ok"`
	URL      string                 `json:"url"`
	Self     SlackAuthenticatedUser `json:"self"`
	Team     SlackTeam              `json:"team"`
	Users    []SlackUser            `json:"users"`
	Channels []SlackChannel         `json:"channels"`
}

// Bot is a representation of the bot and the current status of the slack client.
type Bot struct {
	Conn        *websocket.Conn
	Self        SlackAuthenticatedUser
	Team        SlackTeam
	Users       []SlackUser
	Channels    []SlackChannel
	messageDict map[string][]string
}

// New creates a new bot. This is the proper way to initialize a Bot.
func New() *Bot {
	return &Bot{
		messageDict: map[string][]string{
			"##BOM##": []string{"Hello,"},
			"Hello,":  []string{"World!"},
			"World!":  []string{"##EOM##"},
			"##EOM##": []string{"##EOM##"},
		},
	}
}

// Connect trys to connect the bot to the slack channel via token. It also stores the initial state of the slack.
func (b *Bot) Connect(token string) error {
	c := &http.Client{}
	resp, err := c.Get("https://slack.com/api/rtm.start?token=" + token)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(resp.Body)
	var val rtmStart
	if err := dec.Decode(&val); err != nil {
		return err
	}
	resp.Body.Close()
	u, err := url.Parse(val.URL)
	if err != nil {
		return err
	}
	u.Host += ":443"
	conn, err := websocket.Dial(u.String(), "", "https://slack.com")
	if err != nil {
		return err
	}
	b.Channels = val.Channels
	b.Conn = conn
	b.Users = val.Users
	b.Self = val.Self
	b.Team = val.Team
	return nil
}

// Run starts the bot. It will listen for messages and ping every 20 seconds.
// TODO: add capabilities to parse and send messages.
func (b *Bot) Run() {
	ch := time.Tick(20 * time.Second)
	rec := make(chan *SlackEvent)
	for {
		go func() {
			data := &SlackEvent{}
			websocket.JSON.Receive(b.Conn, data)
			rec <- data
		}()
		select {
		case <-ch:
			websocket.JSON.Send(b.Conn, &struct {
				ID   int    `json:"id"`
				Type string `json:"type"`
			}{
				ID:   2,
				Type: "ping",
			})
		case data := <-rec:
			if data.Type == "message" && b.parse(data) {
				websocket.JSON.Send(b.Conn, b.RandomMessage())
			}
		}
	}
}

func (b *Bot) parse(data *SlackEvent) bool {
	fmt.Println(data)
	words := strings.Split(data.Text, " ")
	b.messageDict["##BOM##"] = append(b.messageDict["##BOM##"], words[0])
	for i := 0; i < len(words)-1; i++ {
		b.messageDict[words[i]] = append(b.messageDict[words[i]], words[i+1])
	}
	b.messageDict[words[len(words)-1]] = append(b.messageDict[words[len(words)-1]], "##EOM##")
	return strings.Contains(data.Text, "<@"+b.Self.ID+">")
}

// RandomMessage returns a randomly generated slack message to send to the server.
func (b *Bot) RandomMessage() SlackMessage {
	token := "##BOM##"
	token = b.messageDict[token][rand.Intn(len(b.messageDict[token]))]
	message := token
	for token != "##EOM##" {
		token = b.messageDict[token][rand.Intn(len(b.messageDict[token]))]
		if token != "##EOM##" {
			message += " " + token
		}
	}

	fmt.Println(message)
	return SlackMessage{
		ID:      1,
		Type:    "message",
		Channel: "C08K8V7GV",
		Text:    message,
	}
}

func main() {
	flag.Parse()

	bot := New()

	err := bot.Connect(*token)
	if err != nil {
		log.Fatal(err)
	}

	websocket.JSON.Send(bot.Conn, bot.RandomMessage())
	bot.Run()
}
