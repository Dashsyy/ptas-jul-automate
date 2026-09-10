package main

import (
	"strings"
	"sync/atomic"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var nextUpdateID int64

func nextID() int {
	return int(atomic.AddInt64(&nextUpdateID, 1))
}

// buildMessageUpdate mimics what Telegram would send for a plain text message
// or a "/command args" message from the simulated owner, including the
// bot_command entity real commands rely on for Message.IsCommand()/Command().
func buildMessageUpdate(chatID, userID int64, text string) tgbotapi.Update {
	msg := &tgbotapi.Message{
		MessageID: nextID(),
		From:      &tgbotapi.User{ID: userID, FirstName: "You"},
		Chat:      &tgbotapi.Chat{ID: chatID, Type: "private"},
		Text:      text,
	}

	if strings.HasPrefix(text, "/") {
		cmdWord := text
		if idx := strings.IndexByte(text, ' '); idx != -1 {
			cmdWord = text[:idx]
		}
		msg.Entities = []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len(cmdWord)},
		}
	}

	return tgbotapi.Update{UpdateID: nextID(), Message: msg}
}

func buildCallbackUpdate(chatID, userID int64, data string) tgbotapi.Update {
	return tgbotapi.Update{
		UpdateID: nextID(),
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "sim",
			From: &tgbotapi.User{ID: userID, FirstName: "You"},
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID, Type: "private"},
			},
			Data: data,
		},
	}
}
