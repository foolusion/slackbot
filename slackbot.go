package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/websocket"
)

var token = flag.String("token", "", "token for the api")

type slackError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

type slackEvent struct {
	OK        bool       `json:"ok"`
	Type      string     `json:"type"`
	Error     slackError `json:"error"`
	Timestamp string     `json:"ts"`
	Text      string     `json:"text"`
	Channel   string     `json:"channel"`
}

type slackAuthenticatedUser struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Created        int    `json:"created"`
	ManualPresence string `json:"manual_presence"`
}

type slackTeam struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	EmailDomain              string `json:"email_domain"`
	MessageEditWindowMinutes int    `json:"msg_edit_window_mins"`
	OverStorageLimit         bool   `json:"over_storage_limit"`
	Plan                     string `json:"plan"`
}

type slackUser struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Deleted           bool             `json:"deleted"`
	Color             string           `json:"color"`
	Profile           slackUserProfile `json:"profile"`
	IsAdmin           bool             `json:"is_admin"`
	IsOwner           bool             `json:"is_owner"`
	IsPrimaryOwner    bool             `json:"is_primary_owner"`
	IsRestricted      bool             `json:"is_restricted"`
	IsUltraRestricted bool             `json:"is_ultra_restricted"`
	HasTwoFactorAuth  bool             `json:"has_2fa"`
	HasFiles          bool             `json:"has_files"`
}

type slackUserProfile struct {
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

type slackChannel struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	IsChannel          bool              `json:"is_channel"`
	Created            int               `json:"created"`
	Creator            string            `json:"creator"`
	IsArchived         bool              `json"is_archived"`
	IsGeneral          bool              `json:"is_general"`
	Members            []string          `json:"members"`
	Topic              slackChannelTopic `json:"topic"`
	Purpose            slackChannelTopic `json:"purpose"`
	IsMember           bool              `json:"is_member"`
	LastRead           string            `json:"last_read"`
	Latest             json.RawMessage   `json:"latest"`
	UnreadCount        int               `json:"unread_count"`
	UnreadCountDisplay int               `json:"unread_count_display"`
}

type slackChannelTopic struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int    `json:"last_set"`
}

type rtmStart struct {
	Ok       bool                   `json:"ok"`
	URL      string                 `json:"url"`
	Self     slackAuthenticatedUser `json:"self"`
	Team     slackTeam              `json:"team"`
	Users    []slackUser            `json:"users"`
	Channels []slackChannel         `json:"channels"`
}

func main() {
	flag.Parse()

	ws, err := Connect(*token)
	if err != nil {
		log.Fatal(err)
	}

	var hello = struct {
		ID      int    `json:"id"`
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}{
		ID:      1,
		Type:    "message",
		Channel: "C08K8V7GV",
		Text:    "Hello, World!",
	}
	websocket.JSON.Send(ws, hello)
	Run(ws)
}

func Connect(token string) (*websocket.Conn, error) {
	c := &http.Client{}
	resp, err := c.Get("https://slack.com/api/rtm.start?token=" + token)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	var val rtmStart
	if err := dec.Decode(&val); err != nil {
		return nil, err
	}
	resp.Body.Close()
	fmt.Printf("%+v\n", val)
	u, err := url.Parse(val.URL)
	if err != nil {
		return nil, err
	}
	u.Host += ":443"
	conn, err := websocket.Dial(u.String(), "", "https://slack.com")
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func Run(ws *websocket.Conn) {
	ch := time.Tick(20 * time.Second)
	rec := make(chan *slackEvent)
	for {
		go func() {
			data := &slackEvent{}
			websocket.JSON.Receive(ws, data)
			rec <- data
		}()
		select {
		case <-ch:
			websocket.JSON.Send(ws, &struct {
				ID   int    `json:"id"`
				Type string `json:"type"`
			}{
				ID:   2,
				Type: "ping",
			})
		case data := <-rec:
			fmt.Println(data)
		}
	}
}
