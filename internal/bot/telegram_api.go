package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type tgClient struct {
	baseURL string
	client  *http.Client
}

func newTGClient(token string) *tgClient {
	return &tgClient{
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
		client:  &http.Client{Timeout: 70 * time.Second},
	}
}

type tgUpdateResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgSendResponse struct {
	OK bool `json:"ok"`
}

type tgUpdate struct {
	UpdateID int64     `json:"update_id"`
	Message  tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int64       `json:"message_id"`
	Chat      tgChat      `json:"chat"`
	From      tgUser      `json:"from"`
	Text      string      `json:"text"`
	Location  *tgLocation `json:"location"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgUser struct {
	ID int64 `json:"id"`
}

type tgLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (c *tgClient) getUpdates(ctx context.Context, offset int64) ([]tgUpdate, error) {
	v := url.Values{}
	v.Set("timeout", "60")
	if offset > 0 {
		v.Set("offset", strconv.FormatInt(offset, 10))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/getUpdates?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed tgUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram getUpdates returned ok=false")
	}
	return parsed.Result, nil
}

func (c *tgClient) sendMessage(ctx context.Context, chatID int64, text string) error {
	body := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	return c.sendMessageRaw(ctx, body)
}

func (c *tgClient) sendMessageWithKeyboard(ctx context.Context, chatID int64, text string, commands []string) error {
	keyboard := make([][]map[string]string, 0, len(commands))
	for i := 0; i < len(commands); i += 2 {
		row := []map[string]string{{"text": commands[i]}}
		if i+1 < len(commands) {
			row = append(row, map[string]string{"text": commands[i+1]})
		}
		keyboard = append(keyboard, row)
	}

	body := map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"keyboard":          keyboard,
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}
	return c.sendMessageRaw(ctx, body)
}

func (c *tgClient) sendLocationRequest(ctx context.Context, chatID int64, text, buttonText string) error {
	if buttonText == "" {
		buttonText = "Отправить текущую геопозицию"
	}
	body := map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"keyboard": [][]map[string]any{
				{
					{
						"text":             buttonText,
						"request_location": true,
					},
				},
			},
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}
	return c.sendMessageRaw(ctx, body)
}

func (c *tgClient) sendMessageRaw(ctx context.Context, body map[string]any) error {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sendMessage", buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var parsed tgSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("telegram sendMessage returned ok=false")
	}
	return nil
}
