package telegrambot

import (
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"log"
	"os"
)

const ChannelsFile = "data/channels.json"

type ChannelsFileData struct {
	ChannelsMap ChannelsFileMap
}

type ChannelsFileMap map[string]ChannelFileList

type ChannelFileList []ChannelInfo

type ChannelInfo struct {
	ChatID    int64  `json:"chat_id"`
	BlockSlug string `json:"block_slug"` // real estate project, e.g 2ngt, utnv
}

func init() {
	// read file, append to hardcode
	channels, err := ReadChannelStorage(ChannelsFile)
	if err != nil {
		log.Printf("unable to read channels file: %v", err)
		return
	}

	err = MergeChannelsWithHardcode(channels)
	if err != nil {
		log.Printf("unable to merge channels file into hardcode: %v", err)
		return
	}
}

func ReadChannelStorage(fileName string) (*ChannelsFileData, error) {
	chnData := &ChannelsFileData{}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	} else {
		// unmarshal the map into json
		err = json.Unmarshal(content, &chnData.ChannelsMap)
		if err != nil {
			return nil, err
		}
	}

	return chnData, nil
}

func MergeChannelsWithHardcode(channels *ChannelsFileData) error {
	if channels == nil {
		return fmt.Errorf("nothing to merge into hardcode: channels == nil")
	}
	if len(channels.ChannelsMap) == 0 {
		return fmt.Errorf("nothing to merge into hardcode: channel map is empty")
	}
	for envTypeStr, channelList := range channels.ChannelsMap {
		envType, ok := util.EnvTypeFromString[envTypeStr]
		if !ok {
			return fmt.Errorf("unknown envtype: %v", envTypeStr)
		}

		oldList := ChannelIDs[envType]
		oldList = append(oldList, channelList...)

		oldList = util.FilterUnique(oldList, func(i int) string {
			return fmt.Sprintf("%v_%v", oldList[i].BlockSlug, oldList[i].ChatID)
		})

		ChannelIDs[envType] = oldList
	}
	return nil
}

func SyncChannelStorageToFile() error {
	channelsFile := &ChannelsFileData{
		ChannelsMap: make(ChannelsFileMap, 10),
	}
	for envtype, channels := range ChannelIDs {
		channelsFile.ChannelsMap[envtype.String()] = channels
	}
	newContent, err := json.Marshal(channelsFile.ChannelsMap)
	if err != nil {
		return err
	}
	err = os.WriteFile(ChannelsFile, newContent, 0644)
	if err != nil {
		return err
	}
	return nil
}
