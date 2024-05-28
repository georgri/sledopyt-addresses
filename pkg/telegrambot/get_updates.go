package telegrambot

import (
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	getUpdatesErrorCooldown      = 10 * time.Second
	getUpdatesPollTimeoutSeconds = 300
	getUpdatesLimitMessages      = 100
)

var LatestKnownUpdateID int64

// how to set up a command suggestions:
// https://core.telegram.org/bots/api#setmycommands

// BotUpdatesStruct Struct to get messages updates for bot, example json:
// {"ok":true,"result":[{"update_id":231999257,
// "channel_post":{"message_id":246,"sender_chat":{"id":-1002057808675,"title":"Pik checker bot tester","username":"pik_checker_bot_tester","type":"channel"},"chat":{"id":-1002057808675,"title":"Pik checker bot tester","username":"pik_checker_bot_tester","type":"channel"},"date":1716055758,"text":"test"}},{"update_id":231999258,
// "message":{"message_id":3,"from":{"id":258990915,"is_bot":false,"first_name":"Georgy","last_name":"Riskov","username":"georgri","language_code":"ru","is_premium":true},"chat":{"id":258990915,"first_name":"Georgy","last_name":"Riskov","username":"georgri","type":"private"},"date":1716055868,"text":"/start","entities":[{"offset":0,"length":6,"type":"bot_command"}]}},{"update_id":231999259,
// "message":{"message_id":4,"from":{"id":258990915,"is_bot":false,"first_name":"Georgy","last_name":"Riskov","username":"georgri","language_code":"ru","is_premium":true},"chat":{"id":258990915,"first_name":"Georgy","last_name":"Riskov","username":"georgri","type":"private"},"date":1716056626,"text":"/hello","entities":[{"offset":0,"length":6,"type":"bot_command"}]}},{"update_id":231999260,
// "message":{"message_id":5,"from":{"id":258990915,"is_bot":false,"first_name":"Georgy","last_name":"Riskov","username":"georgri","language_code":"ru","is_premium":true},"chat":{"id":258990915,"first_name":"Georgy","last_name":"Riskov","username":"georgri","type":"private"},"date":1716056923,"text":"\u043f\u0440\u0438\u0432\u0435\u0442!"}}]}
type UpdateStruct struct {
	UpdateId    int64 `json:"update_id"`
	ChannelPost struct {
		MessageId  int64 `json:"message_id"`
		SenderChat struct {
			Id       int64  `json:"id"`
			Title    string `json:"title"`
			Username string `json:"username"`
			Type     string `json:"type"`
		} `json:"sender_chat"`
		Chat struct {
			Id       int64  `json:"id"`
			Title    string `json:"title"`
			Username string `json:"username"`
			Type     string `json:"type"`
		} `json:"chat"`
		Date int    `json:"date"`
		Text string `json:"text"`
	} `json:"channel_post,omitempty"`
	Message struct {
		MessageId int64 `json:"message_id"`
		From      struct {
			Id           int64  `json:"id"`
			IsBot        bool   `json:"is_bot"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name"`
			Username     string `json:"username"`
			LanguageCode string `json:"language_code"`
			IsPremium    bool   `json:"is_premium"`
		} `json:"from"`
		Chat struct {
			Id        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
			Type      string `json:"type"`
		} `json:"chat"`
		Date     int    `json:"date"`
		Text     string `json:"text"`
		Entities []struct {
			Offset int64  `json:"offset"`
			Length int64  `json:"length"`
			Type   string `json:"type"`
		} `json:"entities,omitempty"`
	} `json:"message,omitempty"`
}

type BotUpdatesStruct struct {
	Ok     bool            `json:"ok"`
	Result []*UpdateStruct `json:"result"`
}

func GetUpdatesForever() {
	log.Printf("polling getUpdates forever...")
	for {
		res, err := GetUpdatesOnce()
		if err != nil {
			log.Printf("error while getting updates: %v", err)
			time.Sleep(getUpdatesErrorCooldown)
			continue
		}
		ProcessUpdates(res)
	}
}

func GetUpdatesOnce() (*BotUpdatesStruct, error) {

	token := util.GetBotToken()

	// test api address to read updates from:
	// https://api.telegram.org/bot6819149165:AAEQWnUotV_YsGS7EPaNbUKZpcvKhsmOgNg/getUpdates
	getUpdatesUrl := fmt.Sprintf("https://api.telegram.org/bot%v/getUpdates", token)

	// also need params (see https://core.telegram.org/bots/api#getting-updates):
	// offset = latest known update_id + 1
	// limit = 100
	// allowed_updates = ["message"]
	// timeout = 300 (seconds)
	values := url.Values{
		"offset":          []string{fmt.Sprintf("%v", LatestKnownUpdateID+1)},
		"limit":           []string{fmt.Sprintf("%v", getUpdatesLimitMessages)},
		"allowed_updates": []string{"message"},
		"timeout":         []string{fmt.Sprintf("%v", getUpdatesPollTimeoutSeconds)},
	}
	// post http request
	resp, err := http.PostForm(getUpdatesUrl, values)
	if err != nil {
		return nil, err
	}

	if resp.ContentLength < 0 {
		return nil, fmt.Errorf("can't read body because content len < 0: %v", resp.Request.URL)
	}

	body := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(body)
	if err != nil {
		return nil, fmt.Errorf("error while reading Body: %v", resp.Request.URL)
	}

	updates := &BotUpdatesStruct{}
	err = json.Unmarshal(body, updates)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling Body: %v", string(body))
	}

	if !updates.Ok {
		return nil, fmt.Errorf("getUpdates response is not OK: %v", string(body))
	}

	return updates, nil
}

func ProcessUpdates(updates *BotUpdatesStruct) {
	if updates == nil {
		return
	}
	for _, update := range updates.Result {
		processUpdate(update)
		LatestKnownUpdateID = util.Max(LatestKnownUpdateID, update.UpdateId)
	}
}

func processUpdate(update *UpdateStruct) {
	if update == nil {
		return
	}
	for _, entity := range update.Message.Entities {
		if entity.Type != "bot_command" {
			continue
		}
		offset, length := entity.Offset, entity.Length
		command := strings.TrimLeft(update.Message.Text[offset:offset+length], "/")

		args := update.Message.Text[offset+length:]
		if strings.Contains(command, "_") {
			command, args, _ = strings.Cut(command, "_")
		}
		args = unEmbedSlug(args)
		switch command {
		case "hello":
			sendHello(update.Message.Chat.Id, update.Message.From.Username)
		case "list":
			sendList(update.Message.Chat.Id)
		case "start":
			sendList(update.Message.Chat.Id)
		case DumpCommand:
			sendDump(update.Message.Chat.Id, args)
		case SubscribeCommand:
			subscribeChat(update.Message.Chat.Id, args)
		case UnsubscribeCommand:
			unsubscribeChat(update.Message.Chat.Id, args)
		}

	}
}
