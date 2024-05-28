package backup_data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const (
	SendFileMethod = "sendDocument"
	BackupChatID   = -1002180492270
)

var BackupBotToken = util.GetBotToken()

type SendFileResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int64  `json:"error_code"`
	Description string `json:"description"`
}

func SendLastBackupFile() error {
	filename, err := GetLastBackupFileName()
	if err != nil {
		return err
	}

	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	cnt := content{
		fname: filename,
		ftype: "document",
		fdata: fileContent,
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%v/%v?chat_id=%v", BackupBotToken, SendFileMethod, BackupChatID)
	resp, err := sendPostRequest(url, cnt)
	if err != nil {
		return err
	}

	r := SendFileResponse{}
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return err
	}

	if r.OK != true {
		return fmt.Errorf("error code: %v, description: %v\n", r.ErrorCode, r.Description)
	}

	return nil
}

// content is a struct which contains a file's name, its type and its data.
type content struct {
	fname string
	ftype string
	fdata []byte
}

func sendPostRequest(url string, files ...content) ([]byte, error) {
	var (
		buf = new(bytes.Buffer)
		w   = multipart.NewWriter(buf)
	)

	for _, f := range files {
		part, err := w.CreateFormFile(f.ftype, filepath.Base(f.fname))
		if err != nil {
			return []byte{}, err
		}

		_, err = part.Write(f.fdata)
		if err != nil {
			return []byte{}, err
		}
	}

	err := w.Close()
	if err != nil {
		return []byte{}, err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return []byte{}, err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()

	cnt, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	return cnt, nil
}
