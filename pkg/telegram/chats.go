package telegram

import (
	"encoding/json"
	"fmt"
	"gopkg.in/tucnak/telebot.v2"
	"strings"
	"time"

	"github.com/docker/libkv/store"
)

const telegramChatsDirectory = "telegram/chats"
const telegramMessagesDirectory = "telegram/messages"

// ChatStore writes the users to a libkv store backend
type ChatStore struct {
	kv store.Store
}

// NewChatStore stores telegram chats in the provided kv backend
func NewChatStore(kv store.Store) (*ChatStore, error) {
	return &ChatStore{kv: kv}, nil
}

// List all chats saved in the kv backend
func (s *ChatStore) List() ([]ChatInfo, error) {
	kvPairs, err := s.kv.List(telegramChatsDirectory)
	if err != nil {
		return nil, err
	}

	var chatInfos []ChatInfo

	for _, kv := range kvPairs {
		var chatInfo ChatInfo
		if err := json.Unmarshal(kv.Value, &chatInfo); err != nil {
			return nil, err
		}
		chatInfos = append(chatInfos, chatInfo)
	}
	return chatInfos, nil
}

func (s *ChatStore) AddChat(c *telebot.Chat, allEnvs []string, allPrs []string) error {
	newChat := ChatInfo{Chat: c,  AlertEnvironments: allEnvs, AlertProjects: allPrs,
		MutedEnvironments: []string{}, MutedProjects: []string{}}
	info, err := json.Marshal(newChat)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	return s.kv.Put(key, info, nil)
}

func (s *ChatStore) AddMessage(m *telebot.Message) error {
	messages, err := s.GetAllMessages()
	if err != nil {
		return err
	}
	messages = append(messages, *m)
	info, err := json.Marshal(messages)
	if err != nil {
		return nil
	}
	return s.kv.Put(telegramMessagesDirectory, info, nil)
}

func (s *ChatStore) GetAllMessages() ([]telebot.Message, error) {
	kvPair, err := s.kv.Get(telegramMessagesDirectory)
	if err != nil {
		if 0 == strings.Compare("Key not found in store", err.Error()) {
			return []telebot.Message{}, nil
		} else {
			return nil, err
		}
	}
	var messages []telebot.Message
	if err = json.Unmarshal(kvPair.Value, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *ChatStore) DeleteAllMessages() error {
	return s.kv.Delete(telegramMessagesDirectory)
}

func (s *ChatStore) GetMessagesForPeriodInMinutes(minutes float64) ([]telebot.Message, error) {
	messages, err := s.GetAllMessages()
	if err != nil {
		return nil, err
	}
	var messagesToDelete []telebot.Message
	var messagesToSave []telebot.Message
	currentTime := time.Now().UTC()
	for _, msg := range messages {
		timeDiff := currentTime.Sub(msg.Time().UTC())
		if timeDiff.Minutes() >= minutes {
			messagesToDelete = append(messagesToDelete, msg)
		} else {
			messagesToSave = append(messagesToSave, msg)
		}
	}
	info, err := json.Marshal(messagesToSave)
	if err != nil {
		return nil, err
	}
	err = s.kv.Put(telegramMessagesDirectory, info, nil)
	if err != nil {
		return nil, err
	}
	return messagesToDelete, nil
}

func (s *ChatStore) GetChatInfo(c *telebot.Chat) (ChatInfo, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return ChatInfo{}, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return ChatInfo{}, err
	}
	return chatInfo, nil
}

func (s *ChatStore) RemoveChat(c *telebot.Chat) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	return s.kv.Delete(key)
}

func (s *ChatStore) MuteEnvironments(c *telebot.Chat, envsToMute []string, allEnvs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.MuteEnvironments(envsToMute, allEnvs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) MuteProjects(c *telebot.Chat, prsToMute []string, allPrs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo *ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.MuteProjects(prsToMute, allPrs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) UnmuteEnvironment(c *telebot.Chat, envToUnmute string, allEnvs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.UnmuteEnvironment(envToUnmute, allEnvs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) UnmuteProject(c *telebot.Chat, prToUnmute string, allPrs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.UnmuteProject(prToUnmute, allPrs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) MutedEnvironments(c *telebot.Chat) ([]string, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return nil, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return nil, err
	}
	return chatInfo.MutedEnvironments, nil
}

func (s *ChatStore) MutedProjects(c *telebot.Chat) ([]string, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return nil, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return nil, err
	}
	return chatInfo.MutedProjects, nil
}