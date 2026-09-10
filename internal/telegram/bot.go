// Package telegram is a thin adapter: it turns Telegram updates into calls
// on service.Service and turns the plain-Go results back into Telegram
// messages/keyboards. It holds no billing logic of its own.
package telegram

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ptas-bot/internal/models"
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
	api      Sender
	svc      *service.Service
	ownerID  int64
	username string // without "@", used to detect @mentions in group chats
}

func New(api Sender, svc *service.Service, ownerID int64, username string) *Bot {
	return &Bot{api: api, svc: svc, ownerID: ownerID, username: username}
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

// authorized restricts every command, callback, and @mention to the
// configured owner's Telegram user ID — full commands only work in a private
// DM (see handleMessage), but the @mention status query is also allowed from
// group chats (e.g. the ABA PayWay notifications group) since it's read-only
// and still gated to this same user ID.
func (b *Bot) authorized(update tgbotapi.Update) bool {
	var userID int64
	switch {
	case update.Message != nil:
		userID = update.Message.From.ID
	case update.CallbackQuery != nil:
		userID = update.CallbackQuery.From.ID
	default:
		return false
	}
	return userID == b.ownerID
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if msg.Chat.Type != "private" {
		// Outside a DM, the only thing we act on is an @mention query
		// (e.g. "@mybot room:4 status") — everything else, including the
		// PayWay bot's own notifications, is ignored here.
		if query, ok := b.mentionQuery(msg); ok {
			b.handleMention(chatID, query)
		}
		return
	}

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

	_, out, err := b.svc.SubmitReadingValue(chatID, value)
	if err != nil {
		if err == service.ErrNoPendingReading {
			return
		}
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	b.reply(chatID, out)
}

func (b *Bot) handleCommand(chatID int64, cmd, args string) {
	switch cmd {
	case "start", "help":
		b.reply(chatID, helpText)

	case "rooms":
		b.sendRoomList(chatID)

	case "unpaid":
		b.sendUnpaidList(chatID)

	case "status":
		b.sendStatusOverview(chatID)

	case "billing":
		b.sendMissingReadingsList(chatID)

	case "newmonth":
		b.doNewMonth(chatID, strings.TrimSpace(args))

	case "pay":
		b.doPay(chatID, args)

	case "setname":
		b.doSetName(chatID, args)

	case "vacate":
		b.doVacate(chatID, strings.TrimSpace(args))

	case "movein":
		b.doMoveIn(chatID, args)

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
		b.reply(chatID, "✅ Settled in full.\n"+formatBillStatus(bill))

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

// floorBreaker writes a "— Floor N —" divider into sb whenever floor differs
// from *lastFloor (including before the very first room), mirroring the
// spreadsheet's per-floor sections.
func floorBreaker(sb *strings.Builder, lastFloor *int, floor int) {
	if floor == *lastFloor {
		return
	}
	fmt.Fprintf(sb, "— Floor %d —\n", floor)
	*lastFloor = floor
}

// floorBreakerRow is floorBreaker for inline-keyboard listings: it returns a
// single-button divider row (tapping it does nothing — "noop" matches no
// callback prefix in handleCallback) whenever floor changes.
func floorBreakerRow(lastFloor *int, floor int) [][]tgbotapi.InlineKeyboardButton {
	if floor == *lastFloor {
		return nil
	}
	*lastFloor = floor
	return [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("— Floor %d —", floor), "noop"),
		),
	}
}

func (b *Bot) sendRoomList(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	var sb strings.Builder
	lastFloor := 0
	for _, r := range rooms {
		floorBreaker(&sb, &lastFloor, r.Floor)
		name := r.TenantName
		switch {
		case r.IsVacant:
			name = "🚪 VACANT"
		case name == "":
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
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		amount := bl.TotalUSD - bl.PaidUSD
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("Room %d — $%.2f", bl.RoomNumber, amount),
				fmt.Sprintf("paid:%d", bl.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Tap a room to settle it in full (use /pay for a partial amount):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// formatBillStatus renders one bill's payment status as a single line —
// shared by /status, the @mention query, and /pay's confirmation.
func formatBillStatus(bl models.Bill) string {
	switch {
	// no_charge is checked before the "readings not entered" case: a vacant
	// room will never get readings entered (nobody's there using water or
	// electricity), so it would otherwise show "readings not entered"
	// forever instead of its actual vacant/no-charge state.
	case bl.Status == models.BillStatusNoCharge && bl.Notes == "vacant":
		return fmt.Sprintf("🚪 Room %d — Vacant", bl.RoomNumber)
	case bl.Status == models.BillStatusNoCharge:
		return fmt.Sprintf("➖ Room %d — No charge", bl.RoomNumber)
	case bl.WaterCurr == nil:
		return fmt.Sprintf("⏳ Room %d — readings not entered", bl.RoomNumber)
	case bl.Status == models.BillStatusPaid:
		if extra := bl.PaidUSD - bl.TotalUSD; extra > 0.01 {
			return fmt.Sprintf("✅ Room %d — Paid ($%.2f due, $%.2f received, $%.2f extra)",
				bl.RoomNumber, bl.TotalUSD, bl.PaidUSD, extra)
		}
		return fmt.Sprintf("✅ Room %d — Paid ($%.2f)", bl.RoomNumber, bl.TotalUSD)
	case bl.Status == models.BillStatusPartial:
		return fmt.Sprintf("🟡 Room %d — Partial ($%.2f of $%.2f, owes $%.2f)",
			bl.RoomNumber, bl.PaidUSD, bl.TotalUSD, bl.TotalUSD-bl.PaidUSD)
	default:
		return fmt.Sprintf("❌ Room %d — Unpaid ($%.2f)", bl.RoomNumber, bl.TotalUSD)
	}
}

func (b *Bot) sendStatusOverview(chatID int64) {
	bills, err := b.svc.AllBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "No rooms in the current period yet.")
		return
	}

	var sb strings.Builder
	lastFloor := 0
	paidCount := 0
	for _, bl := range bills {
		floorBreaker(&sb, &lastFloor, bl.RoomFloor)
		if bl.Status == models.BillStatusPaid {
			paidCount++
		}
		sb.WriteString(formatBillStatus(bl))
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "\n%d/%d rooms paid.", paidCount, len(bills))

	b.reply(chatID, sb.String())
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
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
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

func (b *Bot) doVacate(chatID int64, args string) {
	roomNumber, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		b.reply(chatID, "Usage: /vacate <room#>")
		return
	}
	msg, err := b.svc.StartVacateEntry(chatID, roomNumber)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, msg)
}

func (b *Bot) doMoveIn(chatID int64, args string) {
	roomNumber, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		b.reply(chatID, "Usage: /movein <room#>")
		return
	}
	msg, err := b.svc.StartMoveIn(chatID, roomNumber)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, msg)
}

func (b *Bot) doPay(chatID int64, args string) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) != 2 {
		b.reply(chatID, "Usage: /pay <room#> <amount>")
		return
	}
	roomNumber, err := strconv.Atoi(parts[0])
	if err != nil {
		b.reply(chatID, "Room number must be an integer.")
		return
	}
	amount, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		b.reply(chatID, "Amount must be a number.")
		return
	}

	bill, err := b.svc.PayRoom(roomNumber, amount)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, fmt.Sprintf("💵 Logged $%.2f for Room %d.\n%s", amount, roomNumber, formatBillStatus(bill)))
}

func unwrapFriendly(err error) string {
	if err == service.ErrNoActivePeriod {
		return "No billing period yet — run /newmonth 2026-09 first."
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "Room not found."
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

/status    - show every room's payment status this period
/unpaid    - show unpaid/partial rooms, tap to settle in full
/pay <room#> <amount> - log a partial or full payment
/billing   - show rooms missing readings, tap to enter them
/newmonth YYYY-MM - open a new billing period
/rooms     - list all rooms
/setname <room#> <name> - set a room's tenant name
/vacate <room#> - tenant moving out: auto-prorates to today, asks for final readings, then marks vacant
/movein <room#> - tenant moving in: auto-prorates the rest of the period, asks for starting readings
/cancel    - cancel an in-progress reading entry

In a group, mention me for a read-only room lookup:
@<bot username> room:4 status`
