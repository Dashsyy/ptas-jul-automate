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
//
// Text starting with "@" is treated as a message from a group chat instead
// of the private DM, so typing e.g. "@simbot room:4 status" in the same
// chat box exercises the group @mention code path — the only thing that
// differs from real Telegram is how the chat type gets set, not how Bot
// handles the update.
func buildMessageUpdate(chatID, userID int64, text string) tgbotapi.Update {
	chatType := "private"
	if strings.HasPrefix(text, "@") {
		chatType = "group"
	}

	msg := &tgbotapi.Message{
		MessageID: nextID(),
		From:      &tgbotapi.User{ID: userID, FirstName: "You"},
		Chat:      &tgbotapi.Chat{ID: chatID, Type: chatType},
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
