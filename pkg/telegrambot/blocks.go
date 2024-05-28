package telegrambot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

const BlocksFile = "data/blocks.json"

type BlockInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type BlockInfoMap map[string]BlockInfo

type BlocksFileData struct {
	BlockList []BlockInfo
}

var BlockSlugs BlockInfoMap

func init() {
	BlockSlugs = make(BlockInfoMap, len(BlockSlugSlice))
	for _, blockSlice := range BlockSlugSlice {
		id, err := strconv.Atoi(blockSlice[0])
		if err != nil {
			panic(err)
		}
		slug := strings.Trim(blockSlice[2], "/")
		BlockSlugs[slug] = BlockInfo{
			ID:   int64(id),
			Slug: slug,
			Name: blockSlice[1],
		}
	}
}

func GetBlockIDBySlug(slug string) int64 {
	blockInfo, _ := BlockSlugs[slug]
	return blockInfo.ID
}

func GetBlockURLBySlug(slug string) string {
	return fmt.Sprintf("https://www.pik.ru/%v", slug)
}

func (b BlockInfo) String() string {
	return fmt.Sprintf("%v: <a href=\"%v\">%v</a>", b.Name, GetBlockURLBySlug(b.Slug), b.Slug)
}

func (b BlockInfo) StringWithSub(subscribed bool) string {
	embeddedSlug := embedSlug(b.Slug)
	if subscribed {
		return fmt.Sprintf("✅<a href=\"%v\">%v</a> /%v_%v", GetBlockURLBySlug(b.Slug), b.Name, UnsubscribeCommand, embeddedSlug)
	}
	return fmt.Sprintf("<a href=\"%v\">%v</a> /%v_%v", GetBlockURLBySlug(b.Slug), b.Name, SubscribeCommand, embeddedSlug)
}

func init() {
	// read file, append to hardcode
	blocks, err := ReadBlockStorage(BlocksFile)
	if err != nil {
		log.Printf("unable to read blocks file: %v", err)
		return
	}

	_, err = MergeBlocksWithHardcode(blocks)
	if err != nil {
		log.Printf("unable to merge blocks file into hardcode: %v", err)
		return
	}
}

func ReadBlockStorage(fileName string) (*BlocksFileData, error) {
	blockData := &BlocksFileData{}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	} else {
		// unmarshal the array into json
		err = json.Unmarshal(content, &blockData.BlockList)
		if err != nil {
			return nil, err
		}
	}

	return blockData, nil
}

func MergeBlocksWithHardcode(blocks *BlocksFileData) ([]BlockInfo, error) {
	if blocks == nil {
		return nil, fmt.Errorf("nothing to merge into hardcode: blocks == nil")
	}
	if len(blocks.BlockList) == 0 {
		return nil, fmt.Errorf("nothing to merge into hardcode: block list is empty")
	}
	var newBlocks []BlockInfo
	for _, block := range blocks.BlockList {
		block.Slug = strings.TrimLeft(block.Slug, "/")
		if _, ok := BlockSlugs[block.Slug]; !ok {
			newBlocks = append(newBlocks, block)
		}
		BlockSlugs[block.Slug] = block
	}
	return newBlocks, nil
}

func SyncBlockStorageToFile() error {
	blocks := &BlocksFileData{}
	for _, block := range BlockSlugs {
		blocks.BlockList = append(blocks.BlockList, block)
	}
	newContent, err := json.Marshal(blocks.BlockList)
	if err != nil {
		return err
	}
	err = os.WriteFile(BlocksFile, newContent, 0644)
	if err != nil {
		return err
	}
	return nil
}
