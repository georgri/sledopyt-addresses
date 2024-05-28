package telegrambot

import "github.com/georgri/sledopyt_addresses/pkg/util"

var ChannelIDs = map[util.EnvType][]ChannelInfo{
	util.EnvTypeDev: {
		{
			ChatID:    TestChatID,
			BlockSlug: "2ngt",
		},
		{
			ChatID:    TestChatID,
			BlockSlug: "ytnv",
		},
	},
	util.EnvTypeTesting: {
		{
			ChatID:    TestChatID,
			BlockSlug: "2ngt",
		},
		{
			ChatID:    TestChatID,
			BlockSlug: "ytnv",
		},
	},
	util.EnvTypeProd: {
		{
			ChatID:    -1001451631453,
			BlockSlug: "2ngt",
		},
		{
			ChatID:    -1001439896663,
			BlockSlug: "sp",
		},
		{
			ChatID:    -1002066659264,
			BlockSlug: "ytnv",
		},
		{
			ChatID:    -1002087536270,
			BlockSlug: "kolskaya8",
		},
		{
			ChatID:    -1002123708132,
			BlockSlug: "hp",
		},
	},
}
