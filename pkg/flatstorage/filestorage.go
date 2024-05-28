package flatstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"os"
	"time"
)

const (
	FileMaxUpdatePeriod = 10 * time.Minute

	storageDir    = "data"
	storageFormat = "json"
)

func ReadFlatStorage(fileName string) (*MessageData, error) {
	msgData := &MessageData{}

	if !FileExists(fileName) {
		return msgData, nil
	}

	// read all from file fileFlatStorage
	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	} else {
		// unmarshal into json
		err = json.Unmarshal(content, &msgData)
		if err != nil {
			return nil, err
		}
	}

	return msgData, nil
}

// FilterWithFlatStorage filter through local file (MVP)
func FilterWithFlatStorage(msg *MessageData, chatID int64) (*MessageData, error) {
	if msg == nil || len(msg.Flats) == 0 {
		return msg, nil
	}

	storageFileName := GetStorageFileName(msg, chatID)
	oldMessageData, err := ReadFlatStorage(storageFileName)
	if err != nil {
		return nil, err
	}

	msg = FilterWithFlatStorageHelper(oldMessageData, msg)

	return msg, nil
}

func FilterWithFlatStorageHelper(oldMsg, newMsg *MessageData) *MessageData {
	// gen old map
	oldFlatsMap := make(map[int64]Flat)
	for _, flat := range oldMsg.Flats {
		oldFlatsMap[flat.ID] = flat
	}

	// filter out existing Flats by ID
	newMsg.Flats = util.FilterSliceInPlace(newMsg.Flats, func(i int) bool {
		_, ok := oldFlatsMap[newMsg.Flats[i].ID]
		return !ok
	})

	newMsg.Flats = util.FilterUnique(newMsg.Flats, func(i int) int64 {
		return newMsg.Flats[i].ID
	})

	return newMsg
}

func MergeNewFlatsIntoOld(oldMsg, newMsg *MessageData) *MessageData {
	newMsg.Flats = util.FilterUnique(newMsg.Flats, func(i int) int64 {
		return newMsg.Flats[i].ID
	})

	// gen new map
	newFlatsMap := make(map[int64]struct{})
	for i := range newMsg.Flats {
		newFlatsMap[newMsg.Flats[i].ID] = struct{}{}
	}

	now := time.Now().Format(time.RFC3339)
	past := time.Now().Add(-10 * 365 * 24 * time.Hour).Format(time.RFC3339)

	// old map with created dates
	oldFlatsMap := make(map[int64]string)
	for i := range oldMsg.Flats {
		if len(oldMsg.Flats[i].Created) == 0 {
			oldMsg.Flats[i].Created = past
		}
		if len(oldMsg.Flats[i].Updated) == 0 {
			oldMsg.Flats[i].Updated = oldMsg.Flats[i].Created
		}
		oldFlatsMap[oldMsg.Flats[i].ID] = oldMsg.Flats[i].Created
	}

	// filter out existing old Flats by ID
	oldMsg.Flats = util.FilterSliceInPlace(oldMsg.Flats, func(i int) bool {
		_, ok := newFlatsMap[oldMsg.Flats[i].ID]
		return !ok
	})

	// update both "Created" and "Updated" for new flats
	for i := range newMsg.Flats {
		newMsg.Flats[i].Created = now
		if created, ok := oldFlatsMap[newMsg.Flats[i].ID]; ok {
			newMsg.Flats[i].Created = created
		}
		newMsg.Flats[i].Updated = now
	}

	// dump new into old
	oldMsg.Flats = append(oldMsg.Flats, newMsg.Flats...)

	return oldMsg
}

// UpdateFlatStorage update local file (MVP)
func UpdateFlatStorage(msg *MessageData, chatID int64) (numUpdated int, err error) {
	if msg == nil || len(msg.Flats) == 0 {
		return 0, fmt.Errorf("did not update anything")
	}

	storageFileName := GetStorageFileName(msg, chatID)
	oldMessageData, err := ReadFlatStorage(storageFileName)
	if err != nil {
		return 0, err
	}

	oldMessageData = MergeNewFlatsIntoOld(oldMessageData, msg)

	numUpdated = len(msg.Flats)

	newStorageFileName := GetStorageFileNameByEnv(msg)
	newContent, err := json.Marshal(oldMessageData)
	if err != nil {
		return 0, err
	}
	err = os.WriteFile(newStorageFileName, newContent, 0644)
	if err != nil {
		return 0, err
	}

	return numUpdated, nil
}
func GetStorageFileNameByEnv(msg *MessageData) string {
	blockSlug := msg.GetBlockSlug()
	return GetStorageFileNameByBlockSlugAndEnv(blockSlug)
}

func GetStorageFileName(msg *MessageData, chatID int64) string {
	blockSlug := msg.GetBlockSlug()
	return GetStorageFileNameByBlockSlugAndChatID(blockSlug, chatID)
}

func GetStorageFileNameByBlockSlugAndEnv(blockSlug string) string {
	return fmt.Sprintf("%v/%v_%v.%v", storageDir, blockSlug, util.GetEnvType().String(), storageFormat)
}

func GetStorageFileNameByBlockSlugAndChatID(blockSlug string, chatID int64) string {
	// First, try find file without any chatID but with envtype
	targetFileName := GetStorageFileNameByBlockSlugAndEnv(blockSlug)
	if FileExists(targetFileName) {
		return targetFileName
	}
	fileNameWithChatID := fmt.Sprintf("%v/%v_%v.%v", storageDir, blockSlug, chatID, storageFormat)
	if FileExists(fileNameWithChatID) {
		return fileNameWithChatID
	}
	return targetFileName
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !errors.Is(err, os.ErrNotExist)
}

func FileNotUpdated(filename string) bool {
	stat, err := os.Stat(filename)
	if err != nil {
		return true
	}
	now := time.Now()
	fileModified := stat.ModTime()
	return now.Sub(fileModified) > FileMaxUpdatePeriod
}