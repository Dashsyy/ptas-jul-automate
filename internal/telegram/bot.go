// Package telegram is a thin adapter: it turns Telegram updates into calls
// on service.Service and turns the plain-Go results back into Telegram
// messages/keyboards. It holds no billing logic of its own.
package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ptas-bot/internal/service"
)

// Sender is the subset of *tgbotapi.BotAPI that Bot needs. It exists so a
// local test harness (cmd/simulate) can substitute a fake implementation and
// exercise the exact same HandleUpdate code path a real Telegram webhook or
// long-poll loop would — same authorization, same command parsing, same
// service calls — without talking to Telegram at all.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

type Bot struct {
	api     Sender
	svc     *service.Service
	ownerID int64
}

func New(api Sender, svc *service.Service, ownerID int64) *Bot {
	return &Bot{api: api, svc: svc, ownerID: ownerID}
}

// HandleUpdate is the single entry point used by both the webhook HTTP
// handler and the long-polling loop.
func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	if !b.authorized(update) {
		return
	}

	switch {
	case update.CallbackQuery != nil:
		b.handleCallback(update.CallbackQuery)
	case update.Message != nil:
		b.handleMessage(update.Message)
	}
}

// authorized restricts every mutating command to a private DM from the
// configured owner. Group messages (e.g. future PayWay ingestion) are
// handled separately and aren't gated here.
func (b *Bot) authorized(update tgbotapi.Update) bool {
	var userID int64
	var chatType string
	switch {
	case update.Message != nil:
		userID = update.Message.From.ID
		chatType = update.Message.Chat.Type
	case update.CallbackQuery != nil:
		userID = update.CallbackQuery.From.ID
		if update.CallbackQuery.Message != nil {
			chatType = update.CallbackQuery.Message.Chat.Type
		}
	default:
		return false
	}
	return userID == b.ownerID && chatType == "private"
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if msg.IsCommand() {
		b.handleCommand(chatID, msg.Command(), msg.CommandArguments())
		return
	}

	// Not a command: if a reading is pending for this chat, treat the text
	// as the next meter value. Otherwise ignore.
	text := strings.TrimSpace(msg.Text)
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return
	}

	done, out, err := b.svc.SubmitReadingValue(chatID, value)
	if err != nil {
		if err == service.ErrNoPendingReading {
			return
		}
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	if done {
		b.reply(chatID, "✅ Bill saved.\n\n"+out)
	} else {
		b.reply(chatID, out)
	}
}

func (b *Bot) handleCommand(chatID int64, cmd, args string) {
	switch cmd {
	case "start", "help":
		b.reply(chatID, helpText)

	case "rooms":
		b.sendRoomList(chatID)

	case "unpaid":
		b.sendUnpaidList(chatID)

	case "billing":
		b.sendMissingReadingsList(chatID)

	case "newmonth":
		b.doNewMonth(chatID, strings.TrimSpace(args))

	case "setname":
		b.doSetName(chatID, args)

	case "cancel":
		_ = b.svc.CancelPending(chatID)
		b.reply(chatID, "Cancelled.")

	default:
		b.reply(chatID, "Unknown command. Try /help.")
	}
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	ack := tgbotapi.NewCallback(cb.ID, "")
	if _, err := b.api.Request(ack); err != nil {
		log.Printf("callback ack error: %v", err)
	}

	chatID := cb.Message.Chat.ID
	data := cb.Data

	switch {
	case strings.HasPrefix(data, "paid:"):
		id, err := strconv.ParseInt(strings.TrimPrefix(data, "paid:"), 10, 64)
		if err != nil {
			return
		}
		bill, err := b.svc.MarkPaid(id)
		if err != nil {
			b.reply(chatID, "Error: "+err.Error())
			return
		}
		b.reply(chatID, fmt.Sprintf("✅ Room %d marked paid ($%.2f).", bill.RoomNumber, bill.TotalUSD))

	case strings.HasPrefix(data, "bill:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "bill:"))
		if err != nil {
			return
		}
		prompt, err := b.svc.StartReadingEntry(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, "Error: "+err.Error())
			return
		}
		b.reply(chatID, prompt)
	}
}

func (b *Bot) sendRoomList(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	var sb strings.Builder
	sb.WriteString("Rooms:\n")
	for _, r := range rooms {
		name := r.TenantName
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Fprintf(&sb, "#%d — %s — $%.0f/mo\n", r.Number, name, r.BaseRentUSD)
	}
	b.reply(chatID, sb.String())
}

func (b *Bot) sendUnpaidList(chatID int64) {
	bills, err := b.svc.UnpaidBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "🎉 Everyone's paid up this period.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, bl := range bills {
		label := fmt.Sprintf("Room %d — $%.2f", bl.RoomNumber, bl.TotalUSD)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("paid:%d", bl.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Tap a room to mark it paid:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) sendMissingReadingsList(chatID int64) {
	bills, err := b.svc.RoomsMissingReadings()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "✅ All rooms have readings entered this period.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, bl := range bills {
		label := fmt.Sprintf("Room %d", bl.RoomNumber)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("bill:%d", bl.RoomNumber)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Tap a room to enter its meter readings:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) doNewMonth(chatID int64, label string) {
	if label == "" {
		b.reply(chatID, "Usage: /newmonth 2026-09")
		return
	}
	count, err := b.svc.NewMonth(label)
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	b.reply(chatID, fmt.Sprintf("📅 Opened period %s with %d rooms.", label, count))
}

func (b *Bot) doSetName(chatID int64, args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		b.reply(chatID, "Usage: /setname <room#> <name>")
		return
	}
	roomNumber, err := strconv.Atoi(parts[0])
	if err != nil {
		b.reply(chatID, "Room number must be an integer.")
		return
	}
	if err := b.svc.SetTenantName(roomNumber, parts[1]); err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	b.reply(chatID, fmt.Sprintf("Room %d name set to %q.", roomNumber, parts[1]))
}

func unwrapFriendly(err error) string {
	if err == service.ErrNoActivePeriod {
		return "No billing period yet — run /newmonth 2026-09 first."
	}
	return "Error: " + err.Error()
}

func (b *Bot) reply(chatID int64, text string) {
	b.send(tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) send(msg tgbotapi.MessageConfig) {
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send error: %v", err)
	}
}

const helpText = `PTAS billing bot

/unpaid    - show unpaid rooms this period, tap to mark paid
/billing   - show rooms missing readings, tap to enter them
/newmonth YYYY-MM - open a new billing period
/rooms     - list all rooms
/setname <room#> <name> - set a room's tenant name
/cancel    - cancel an in-progress reading entry`
