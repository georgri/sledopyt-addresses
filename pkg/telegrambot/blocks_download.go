package telegrambot

import (
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/downloader"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"log"
	"sort"
	"strings"
	"time"
)

const (
	BlocksURL = "https://flat.pik-service.ru/api/v1/filter/block?type=1,2&blockLimit=1000&geoBox=1.0,179.0-1.0,179.0"

	UpdateBlocksEvery = 1 * time.Hour
)

type BlockSiteData struct {
	Success bool `json:"success"`
	Data    struct {
		Items []struct {
			Id   int64  `json:"id"`
			Name string `json:"name"`
			Path string `json:"path"` // = slug
		} `json:"items"`
	} `json:"data"`
}

func DownloadBlocks() (*BlocksFileData, error) {
	url := BlocksURL
	body, err := downloader.GetUrl(url)
	if err != nil {
		return nil, fmt.Errorf("error while getting url %v: %v", url, err)
	}

	blockSiteData := &BlockSiteData{}
	err = json.Unmarshal(body, blockSiteData)
	if err != nil {
		return nil, err
	}

	blockData := &BlocksFileData{}
	for _, block := range blockSiteData.Data.Items {
		blockData.BlockList = append(blockData.BlockList, BlockInfo{
			ID:   block.Id,
			Name: block.Name,
			Slug: strings.TrimLeft(block.Path, "/"),
		})
	}

	return blockData, nil
}

func UpdateBlocksForever() {
	for {
		err := UpdateBlocksOnce()
		if err != nil {
			log.Printf("update blocks failed: %v", err)
		}
		time.Sleep(UpdateBlocksEvery)
	}
}

func UpdateBlocksOnce() error {
	blocks, err := DownloadBlocks()
	if err != nil {
		return fmt.Errorf("unable to download blocks: %v", err)
	}

	newBlocks, err := MergeBlocksWithHardcode(blocks)
	if err != nil {
		return fmt.Errorf("unable to merge downloaded blocks: %v", err)
	}

	err = SyncBlockStorageToFile()
	if err != nil {
		return fmt.Errorf("unable to sync blocks to file: %v", err)
	}

	err = NotifyAboutNewBlocks(newBlocks)
	if err != nil {
		return fmt.Errorf("unable to notify about new blocks: %v", err)
	}

	return nil
}

// NotifyAboutNewBlocks notifies all known chats about the new projects
func NotifyAboutNewBlocks(newBlocks []BlockInfo) error {
	if len(newBlocks) == 0 {
		return nil
	}

	sort.Slice(newBlocks, func(i, j int) bool {
		return newBlocks[i].Slug < newBlocks[j].Slug
	})

	var res []string
	res = append(res, "#NewPikProjects\n")
	for _, block := range newBlocks {
		res = append(res, block.String())
	}
	res = append(res, "\nTo follow new updates, write @pik_checker_bot")
	msg := strings.Join(res, "\n")

	err := SendToAllKnownChats(msg)
	if err != nil {
		return err
	}

	return nil
}

func SendToAllKnownChats(msg string) error {
	chatIDs := GetAllKnownChatIDs()
	for _, chatID := range chatIDs {
		err := SendMessage(chatID, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetAllKnownChatIDs() []int64 {
	var res []int64
	for _, chat := range ChannelIDs[util.GetEnvType()] {
		res = append(res, chat.ChatID)
	}
	res = util.FilterUnique(res, func(i int) int64 {
		return res[i]
	})
	return res
}
