package telegrambot

import (
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/flatstorage"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"log"
	"strings"
	"time"
)

const (
	DumpCommand        = "dump"
	SubscribeCommand   = "sub"
	UnsubscribeCommand = "unsub"
)

func sendHello(chatID int64, username string) {
	msg := fmt.Sprintf("Hello, %v!", username)
	err := SendMessage(chatID, msg)
	if err != nil {
		log.Printf("failed to send message %v to chatID %v: %v", msg, chatID, err)
	}
}

func sendList(chatID int64) {

	subscribedTo := GetChatSubscriptions(chatID)

	var complexes []string
	for _, comp := range util.SortedKeys(BlockSlugs) {
		isSubscribed := subscribedTo[comp]
		complexes = append(complexes, BlockSlugs[comp].StringWithSub(isSubscribed))
	}
	msg := fmt.Sprintf("List of known complexes:\n") + strings.Join(complexes, "\n")
	err := SendMessage(chatID, msg)
	if err != nil {
		log.Printf("failed to send list of all blocks to chatID %v: %v", chatID, err)
	}
}

func GetChatSubscriptions(chatID int64) map[string]bool {
	envtype := util.GetEnvType()
	res := make(map[string]bool, 10)
	for _, channel := range ChannelIDs[envtype] {
		if channel.ChatID == chatID {
			res[channel.BlockSlug] = true
		}
	}
	return res
}

func validateSlug(chatID int64, slug string, command string) (string, error) {
	slug = strings.TrimLeft(strings.TrimSpace(slug), "/")

	_, slugIsValid := BlockSlugs[slug]

	if len(slug) == 0 || !slugIsValid {
		// send help message
		err := SendMessage(chatID, fmt.Sprintf("usage: /%v [code]\n\nTo get [code] of any complex type /list", command))
		if err != nil {
			return "", fmt.Errorf("failed to send /%v help message: %v", command, err)
		}
		return "", fmt.Errorf("slug is empty or invalid: %v", slug)
	}
	return slug, nil
}

func sendDump(chatID int64, slug string) {

	slug, err := validateSlug(chatID, slug, DumpCommand)
	if err != nil {
		log.Printf("failed to dump to %v: %v", chatID, err)
		return
	}

	var msg string

	// send all known flats for complex with slug "slug"
	fileName, err := GetStorageFileNameByBlockSlug(slug)
	if !flatstorage.FileExists(fileName) || flatstorage.FileNotUpdated(fileName) {
		// force update flats into file
		msg, err = DownloadAndUpdateFile(slug, 0)
		if err != nil {
			log.Printf("failed to download/update flats for slug %v: %v", slug, err)
			return
		}
	} else {
		allFlatsMessageData, err := flatstorage.ReadFlatStorage(fileName)
		if err != nil {
			log.Printf("failed to read file with flats %v: %v", fileName, err)
			return
		}

		// output recently updated only
		now := time.Now()
		allFlatsMessageData.Flats = util.FilterSliceInPlace(allFlatsMessageData.Flats, func(i int) bool {
			return allFlatsMessageData.Flats[i].RecentlyUpdated(now)
		})

		msg = allFlatsMessageData.String()
		if len(allFlatsMessageData.Flats) == 0 {
			msg = fmt.Sprintf("No known flats for complex %v", slug)
		}
	}

	err = SendMessageWithPin(chatID, msg, true)
	if err != nil {
		log.Printf("failed to send list of all blocks to chatID %v: %v", chatID, err)
	}
}

func GetStorageFileNameByBlockSlug(blockSlug string) (string, error) {
	// guess chatID
	// TODO: go with empty chatID
	var chatID int64
	for _, channel := range ChannelIDs[util.GetEnvType()] {
		if channel.BlockSlug == blockSlug {
			chatID = channel.ChatID
			break
		}
	}
	if chatID == 0 {
		return "", fmt.Errorf("yet unknown block slug: %v", blockSlug)
	}
	return flatstorage.GetStorageFileNameByBlockSlugAndChatID(blockSlug, chatID), nil
}

func AddNewSubscriber(chatID int64, slug string) error {
	envtype := util.GetEnvType()
	ChannelIDs[envtype] = append(ChannelIDs[envtype], ChannelInfo{
		ChatID:    chatID,
		BlockSlug: slug,
	})

	err := SyncChannelStorageToFile()
	if err != nil {
		n := len(ChannelIDs[envtype])
		ChannelIDs[envtype] = ChannelIDs[envtype][:n-1]
		return err
	}

	return nil
}

func RemoveSubscriber(chatID int64, slug string) error {
	envtype := util.GetEnvType()

	indexToRemove := -1
	for i, subscription := range ChannelIDs[envtype] {
		if subscription.BlockSlug == slug && subscription.ChatID == chatID {
			indexToRemove = i
			break
		}
	}
	if indexToRemove < 0 || indexToRemove >= len(ChannelIDs[envtype]) {
		return fmt.Errorf("chat %v was not subscribed to %v", chatID, slug)
	}

	ChannelIDs[envtype] = util.RemoveSliceElement(ChannelIDs[envtype], indexToRemove)

	err := SyncChannelStorageToFile()
	if err != nil {
		return err
	}

	return nil
}

func CheckSubscribed(chatID int64, slug string) bool {
	envtype := util.GetEnvType()

	for _, subscription := range ChannelIDs[envtype] {
		if subscription.BlockSlug == slug && subscription.ChatID == chatID {
			return true
		}
	}
	return false
}

func subscribeChat(chatID int64, slug string) {

	slug, err := validateSlug(chatID, slug, SubscribeCommand)
	if err != nil {
		log.Printf("failed to subscribe %v to slug %v: %v", chatID, slug, err)
		return
	}

	embeddedSlug := embedSlug(slug)

	if CheckSubscribed(chatID, slug) {
		// send already subscribed message
		err = SendMessage(chatID, fmt.Sprintf("You are already subscribed to complex %v.\n"+
			"To view all flats: /%v_%v", slug, DumpCommand, embeddedSlug))
		if err != nil {
			log.Printf("failed to send already subscribed message to %v: %v", chatID, err)
		}
		log.Printf("chat %v is already subscribed to %v", chatID, slug)
		return
	}

	err = AddNewSubscriber(chatID, slug)
	if err != nil {
		// send something went wrong while subscribing message
		err = SendMessage(chatID, fmt.Sprintf("Something went wrong while subscribing to %v:\n"+
			"error: %v\n"+
			"You can try again later with /%v_%v", slug, err, SubscribeCommand, embeddedSlug))
		if err != nil {
			log.Printf("failed to send subscription failed message to %v: %v", chatID, err)
		}
		log.Printf("failed to subscribe %v to %v", chatID, slug)
		return
	}

	// send message "You are subscribed"
	err = SendMessage(chatID, fmt.Sprintf("You are now subscribed to new flats from: %v.\n"+
		"To unsubscribe, click here: /%v_%v\n"+
		"To get all known flats click here: /%v_%v", slug, UnsubscribeCommand, embeddedSlug, DumpCommand, embeddedSlug))
	if err != nil {
		log.Printf("failed to send subscribed message to %v: %v", chatID, err)
	}
}

func unsubscribeChat(chatID int64, slug string) {
	slug, err := validateSlug(chatID, slug, UnsubscribeCommand)
	if err != nil {
		log.Printf("failed to unsubscribe %v from slug %v: %v", chatID, slug, err)
		return
	}

	embeddedSlug := embedSlug(slug)

	if !CheckSubscribed(chatID, slug) {
		// send already subscribed message
		err = SendMessage(chatID, fmt.Sprintf("You are not currently subscribed to complex %v.\n"+
			"To subscribe: /%v_%v\n"+
			"To view all flats: /%v_%v", slug, DumpCommand, embeddedSlug, SubscribeCommand, embeddedSlug))
		if err != nil {
			log.Printf("failed to send already unsubscribed message to %v: %v", chatID, err)
		}
		log.Printf("chat %v is already unsubscribed to %v", chatID, slug)
		return
	}

	err = RemoveSubscriber(chatID, slug)
	if err != nil {
		// send something went wrong while unsubscribing message
		err = SendMessage(chatID, fmt.Sprintf("Something went wrong while unsubscribing from %v:\n"+
			"error: %v\n"+
			"You might need to try again later with /%v_%v", slug, err, UnsubscribeCommand, embeddedSlug))
		if err != nil {
			log.Printf("failed to send unsubscription failed message to %v: %v", chatID, err)
		}
		log.Printf("failed to unsubscribe %v from %v", chatID, slug)
		return
	}

	// send message "You are unsubscribed"
	err = SendMessage(chatID, fmt.Sprintf("You were unsubscribed from: %v.\n"+
		"To subscribe again, click here: /%v_%v\n"+
		"To get all known flats click here: /%v_%v", slug, SubscribeCommand, embeddedSlug, DumpCommand, embeddedSlug))
	if err != nil {
		log.Printf("failed to send unsubscribed message to %v: %v", chatID, err)
	}
}

func embedSlug(slug string) string {
	return strings.ReplaceAll(strings.ReplaceAll(slug, "/", "__"), "-", "_")
}

func unEmbedSlug(slug string) string {
	return strings.ReplaceAll(strings.ReplaceAll(slug, "__", "/"), "_", "-")
}
