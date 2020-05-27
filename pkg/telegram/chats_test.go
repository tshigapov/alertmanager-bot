package telegram

import (
	"fmt"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/stretchr/testify/assert"
	"gopkg.in/tucnak/telebot.v2"
	"os"
	"testing"
	"time"
)

var bot *Bot

func TestMain(m *testing.M) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	var kvStore store.Store
	{
		var err error
		kvStore, err  = boltdb.New([]string{"/tmp/bot.db"}, &store.Config{Bucket: "alertmanager"})
		if err != nil {
			level.Error(logger).Log("msg", "failed to create bolt store backend", "err", err)
		}
	}
	defer kvStore.Close()

	chats, err := NewChatStore(kvStore)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create chat store", "err", err)
		os.Exit(1)
	}

	bot = &Bot{chats:chats}

	if err != nil {
		level.Error(logger).Log("msg", "failed to create bot", "err", err)
		os.Exit(2)
	}
	code := m.Run()
	os.Exit(code)
}

func TestMutingEnvironment(t *testing.T) {
	allEnvs := []string{"env1", "env2", "env3"}
	allPrs := []string{"pr1", "pr2"}
	chat := telebot.Chat{ID:123}
	err := bot.chats.AddChat(&chat, allEnvs, allPrs)
	assert.Nil(t, err)

	err = bot.chats.MuteEnvironments(&chat, []string{"env1"}, allEnvs)
	assert.Nil(t, err)

	chatInfo, err := bot.chats.GetChatInfo(&chat)
	assert.Nil(t, err)
	assert.True(t, len(chatInfo.AlertEnvironments) == 2)
	assert.True(t, len(chatInfo.MutedEnvironments) == 1)
}

func TestMutingProjects(t *testing.T) {
	allEnvs := []string{"env1", "env2", "env3"}
	allPrs := []string{"pr1", "pr2"}
	chat := telebot.Chat{ID:1233}
	err := bot.chats.AddChat(&chat, allEnvs, allPrs)
	assert.Nil(t, err)

	err = bot.chats.MuteProjects(&chat, []string{"pr1"}, allPrs)
	assert.Nil(t, err)

	chatInfo, err := bot.chats.GetChatInfo(&chat)
	assert.Nil(t, err)
	assert.True(t, len(chatInfo.AlertProjects) == 1)
	assert.True(t, len(chatInfo.MutedProjects) == 1)
}

func TestUnmuteEnvironment(t *testing.T) {
	allEnvs := []string{"env1", "env2", "env3"}
	allPrs := []string{"pr1", "pr2"}
	chat := telebot.Chat{ID:134}
	err := bot.chats.AddChat(&chat, allEnvs, allPrs)
	assert.Nil(t, err)

	err = bot.chats.MuteEnvironments(&chat, []string{"env1", "env2"}, allEnvs)
	assert.Nil(t, err)

	chatInfo, err := bot.chats.GetChatInfo(&chat)
	assert.Nil(t, err)
	assert.True(t, len(chatInfo.AlertEnvironments) == 1)
	assert.True(t, len(chatInfo.MutedEnvironments) == 2)

	err = bot.chats.UnmuteEnvironment(&chat, "env1", allEnvs)
	assert.Nil(t, err)

	chatInfo, err = bot.chats.GetChatInfo(&chat)
	assert.Nil(t, err)
	assert.True(t, len(chatInfo.MutedEnvironments) == 1)
	assert.True(t, len(chatInfo.AlertEnvironments) == 2)
}

func TestGettingChatLists(t *testing.T) {
	allEnvs := []string{"env1", "env2", "env3"}
	allPrs := []string{"pr1", "pr2"}
	chat := telebot.Chat{ID:134}
	err := bot.chats.AddChat(&chat, allEnvs, allPrs)
	assert.Nil(t, err)

	chat = telebot.Chat{ID:32}
	err = bot.chats.AddChat(&chat, allEnvs, allPrs)
	assert.Nil(t, err)

	chats, err := bot.chats.List()
	assert.Nil(t, err)
	for _, chat := range chats {
		fmt.Println(chat)
	}
}

func TestAddingMessage(t *testing.T) {
	bot.chats.DeleteAllMessages()
	msg := telebot.Message{ID: 2424}
	err := bot.chats.AddMessage(&msg)
	assert.Nil(t, err)
	messages, err := bot.chats.GetAllMessages()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(messages))

	msg.ID = 12
	err = bot.chats.AddMessage(&msg)
	assert.Nil(t, err)
	messages, err = bot.chats.GetAllMessages()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(messages))
}

func TestGettingMessagesForPeriod(t *testing.T) {
	bot.chats.DeleteAllMessages()
	msg := telebot.Message{ID: 244, Unixtime: time.Now().UTC().Unix()}
	err := bot.chats.AddMessage(&msg)
	assert.Nil(t, err)
	msgsToDelete, err := bot.chats.GetMessagesForPeriodInMinutes(1)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(msgsToDelete))

	time.Sleep(1 * time.Minute)

	msg = telebot.Message{ID: 44, Unixtime: time.Now().UTC().Unix()}
	err = bot.chats.AddMessage(&msg)
	assert.Nil(t, err)
	msgsToDelete, err = bot.chats.GetMessagesForPeriodInMinutes(1)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(msgsToDelete))
	msgsSaved, err := bot.chats.GetAllMessages()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(msgsSaved))

}