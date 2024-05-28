package telegrambot

import (
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"net/http"
	"net/url"
	"strings"
)

const (
	TestChatID = -1002057808675

	messageCharLimit = 4000
)

// An example of how to send message with test bot:
// https://api.telegram.org/bot6819149165:AAEQWnUotV_YsGS7EPaNbUKZpcvKhsmOgNg/sendMessage?chat_id=-1002057808675&text=hello_friend
// i.e. https://api.telegram.org/bot{token}/sendMessage?chat_id={chat_id}&text={text}

func SendTestMessage(text string) error {
	return SendMessage(TestChatID, text)
}

func SendMessage(chatID int64, text string) error {
	return SendMessageWithPin(chatID, text, false)
}

func SendMessageWithPin(chatID int64, text string, mustPin bool) error {
	token := util.GetBotToken()

	chunks := SplitTextIntoSendableChunks(text)

	var messageIDToDefer int64
	for i, msg := range chunks {
		messageID, err := SendMessageWithToken(token, chatID, msg)
		if err != nil {
			return err
		}
		if len(chunks) > 1 && i == 0 && mustPin {
			messageIDToDefer = messageID
		}
	}

	if mustPin && messageIDToDefer != 0 {
		err := PinMessage(token, chatID, messageIDToDefer)
		if err != nil {
			return err
		}
	}

	return nil
}

type PinResponse struct {
	OK          bool   `json:"ok"`
	Result      bool   `json:"result"`
	ErrorCode   int64  `json:"error_code"`
	Description string `json:"description"`
}

func PinMessage(token string, chatID int64, messageID int64) error {
	pinMessageUrl := fmt.Sprintf("https://api.telegram.org/bot%v/pinChatMessage", token)

	values := url.Values{
		"chat_id":    []string{fmt.Sprintf("%v", chatID)},
		"message_id": []string{fmt.Sprintf("%v", messageID)},
		//"disable_notification": []string{"False"},
	}

	// post http request
	resp, err := http.PostForm(pinMessageUrl, values)
	if err != nil {
		return err
	}

	if resp.ContentLength < 0 {
		return fmt.Errorf("can't read body because content len < 0: %v", resp.Request.URL)
	}

	// example of response:
	// {"ok":true"}}
	body := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(body)
	if err != nil {
		return fmt.Errorf("error while reading Body: %v", resp.Request.URL)
	}

	pinResponse := &PinResponse{}
	err = json.Unmarshal(body, pinResponse)
	if err != nil {
		return fmt.Errorf("error while unmarshalling Body: %v", string(body))
	}

	if !pinResponse.OK || !pinResponse.Result {
		return fmt.Errorf("pin response is not OK: %v", string(body))
	}

	return nil
}

type SendResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		MessageId int64 `json:"message_id"`
	} `json:"result"`
}

func SendMessageWithToken(token string, chatID int64, text string) (int64, error) {

	sendMessageUrl := fmt.Sprintf("https://api.telegram.org/bot%v/sendMessage", token)

	values := url.Values{
		"chat_id":                  []string{fmt.Sprintf("%v", chatID)},
		"text":                     []string{text},
		"parse_mode":               []string{"HTML"},
		"disable_web_page_preview": []string{"True"},
	}
	// post http request
	resp, err := http.PostForm(sendMessageUrl, values)
	if err != nil {
		return 0, err
	}

	if resp.ContentLength < 0 {
		return 0, fmt.Errorf("can't read body because content len < 0: %v", resp.Request.URL)
	}

	// example of response:
	// {"ok":true,"result":{"message_id":5,"sender_chat":{"id":-1002057808675,"title":"Pik checker bot tester","username":"pik_checker_bot_tester","type":"channel"},"chat":{"id":-1002057808675,"title":"Pik checker bot tester","username":"pik_checker_bot_tester","type":"channel"},"date":1701824824,"text":"hello_friend"}}
	body := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(body)
	if err != nil {
		return 0, fmt.Errorf("error while reading Body: %v", resp.Request.URL)
	}

	sendResponse := &SendResponse{}
	err = json.Unmarshal(body, sendResponse)
	if err != nil {
		return 0, fmt.Errorf("error while unmarshalling Body: %v", string(body))
	}

	if !sendResponse.OK {
		return 0, fmt.Errorf("send response is not OK: %v", string(body))
	}

	return sendResponse.Result.MessageId, nil
}

func SplitTextIntoSendableChunks(text string) []string {
	lines := strings.Split(text, "\n")

	res := make([]string, 0, 5)
	var bufFrom, bufSize int
	for i, line := range lines {
		if bufSize+len(line) > messageCharLimit {
			res = append(res, strings.Join(lines[bufFrom:i], "\n"))
			bufFrom = i
			bufSize = 0
		}
		bufSize += len(line) + 1
	}

	res = append(res, strings.Join(lines[bufFrom:], "\n"))

	return res
}
