package main

import (
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// OutMessage is what the browser-side chat UI renders for one bot reply.
type OutMessage struct {
	Text    string   `json:"text"`
	Buttons []Button `json:"buttons,omitempty"`
}

type Button struct {
	Label string `json:"label"`
	Data  string `json:"data"`
}

// fakeSender implements telegram.Sender by recording what the bot would have
// sent instead of calling the real Telegram API, so the simulate HTTP
// handlers can hand the recorded messages straight back to the browser.
type fakeSender struct {
	mu     sync.Mutex
	outbox []OutMessage
}

func newFakeSender() *fakeSender {
	return &fakeSender{}
}

func (f *fakeSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if m, ok := c.(tgbotapi.MessageConfig); ok {
		out := OutMessage{Text: m.Text}
		if kb, ok := m.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
			for _, row := range kb.InlineKeyboard {
				for _, btn := range row {
					data := ""
					if btn.CallbackData != nil {
						data = *btn.CallbackData
					}
					out.Buttons = append(out.Buttons, Button{Label: btn.Text, Data: data})
				}
			}
		}
		f.mu.Lock()
		f.outbox = append(f.outbox, out)
		f.mu.Unlock()
	}
	return tgbotapi.Message{}, nil
}

// Request handles the callback-query "answer" ack and anything else the bot
// sends via Request. The simulate UI doesn't need to show anything for it.
func (f *fakeSender) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{Ok: true}, nil
}

// Drain returns everything sent since the last call and clears the buffer.
func (f *fakeSender) Drain() []OutMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.outbox
	f.outbox = nil
	if out == nil {
		out = []OutMessage{}
	}
	return out
}
