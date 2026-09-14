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

// commandList is registered with Telegram via setMyCommands so typing "/" in
// the chat shows this as an autocomplete menu — the same discovery
// mechanism BotFather itself relies on.
var commandList = []tgbotapi.BotCommand{
	{Command: "start", Description: "បើកម៉ឺនុយដើម"},
	{Command: "status", Description: "របាយការណ៍បង់ប្រាក់ប្រចាំខែ"},
	{Command: "unpaid", Description: "បន្ទប់ជំពាក់ (ចុចដើម្បីទូទាត់)"},
	{Command: "pay", Description: "កត់ត្រាការបង់ប្រាក់"},
	{Command: "billing", Description: "បន្ទប់មិនទាន់កត់លេខម៉ែត្រ"},
	{Command: "rooms", Description: "មើលបញ្ជីបន្ទប់ទាំងអស់"},
	{Command: "setname", Description: "ដាក់ឈ្មោះអ្នកជួល"},
	{Command: "vacate", Description: "កត់ត្រាអ្នករើចេញ"},
	{Command: "movein", Description: "កត់ត្រាអ្នករើចូល"},
	{Command: "newmonth", Description: "បង្កើតវិក្កយបត្រខែថ្មី"},
	{Command: "cancel", Description: "បោះបង់"},
	{Command: "help", Description: "របៀបប្រើប្រាស់រូបូត"},
}

// RegisterCommands pushes commandList to Telegram. Call it once at startup;
// it's harmless (and cheap) to call again on every restart.
func (b *Bot) RegisterCommands() error {
	_, err := b.api.Request(tgbotapi.NewSetMyCommands(commandList...))
	return err
}

// SetCommandsMenuButton sets the hamburger-style menu button shown next to
// the message box (bottom-left) to open the registered command list — the
// same discovery affordance most Telegram bots ship with. The library
// doesn't wrap Telegram's setChatMenuButton API, so this calls it directly
// via the concrete *tgbotapi.BotAPI (a one-time startup call, so it doesn't
// need to go through the Sender interface used for per-update messaging).
func SetCommandsMenuButton(api *tgbotapi.BotAPI) error {
	params := tgbotapi.Params{}
	if err := params.AddInterface("menu_button", map[string]string{"type": "commands"}); err != nil {
		return err
	}
	_, err := api.MakeRequest("setChatMenuButton", params)
	return err
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

	// Not a command: if some conversation is pending for this chat (a meter
	// reading, a payment amount, a tenant name, a new period label...),
	// treat the text as the reply to it. Otherwise ignore.
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	_, out, err := b.svc.SubmitTextReply(chatID, text)
	if err != nil {
		if err == service.ErrNoPendingReading {
			return
		}
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	b.reply(chatID, out)
}

func (b *Bot) handleCommand(chatID int64, cmd, args string) {
	switch cmd {
	case "start":
		b.showMainMenu(chatID)

	case "help":
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
		b.reply(chatID, "ប្រតិបត្តិការត្រូវបានបោះបង់។")

	default:
		b.reply(chatID, "ខ្ញុំមិនស្គាល់ពាក្យបញ្ជានេះទេ។ សូមសាកល្បងវាយ /help។")
	}
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	ack := tgbotapi.NewCallback(cb.ID, "")
	if _, err := b.api.Request(ack); err != nil {
		log.Printf("callback ack error: %v", err)
	}

	chatID := cb.Message.Chat.ID
	data := cb.Data

	if b.handleMenuCallback(chatID, data) {
		return
	}

	switch {
	case strings.HasPrefix(data, "paid:"):
		id, err := strconv.ParseInt(strings.TrimPrefix(data, "paid:"), 10, 64)
		if err != nil {
			return
		}
		bill, err := b.svc.MarkPaid(id)
		if err != nil {
			b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
			return
		}
		b.reply(chatID, "✅ ទូទាត់រួចរាល់។\n"+formatBillStatus(bill))

	case strings.HasPrefix(data, "bill:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "bill:"))
		if err != nil {
			return
		}
		prompt, err := b.svc.StartReadingEntry(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
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
	fmt.Fprintf(sb, "— ជាន់ទី %d —\n", floor)
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
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("— ជាន់ទី %d —", floor), "noop"),
		),
	}
}

func (b *Bot) sendRoomList(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	var sb strings.Builder
	lastFloor := 0
	for _, r := range rooms {
		floorBreaker(&sb, &lastFloor, r.Floor)
		status := "✅ មានអ្នកជួល"
		if r.IsVacant {
			status = "🚪 ទំនេរ"
		}
		fmt.Fprintf(&sb, "បន្ទប់ %d — %s — $%.0f/ខែ\n", r.Number, status, r.BaseRentUSD)
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
		b.reply(chatID, "🎉 គ្រប់គ្នាបានបង់លុយគ្រប់ចំនួនសម្រាប់ខែនេះហើយ។")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		amount := bl.TotalUSD - bl.PaidUSD
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("បន្ទប់ %d — $%.2f", bl.RoomNumber, amount),
				fmt.Sprintf("paid:%d", bl.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "ចុចលើបន្ទប់ដើម្បីទូទាត់ប្រាក់ (ប្រើ /pay សម្រាប់បង់ខ្លះ)៖")
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
		return fmt.Sprintf("🚪 បន្ទប់ %d — ទំនេរ", bl.RoomNumber)
	case bl.Status == models.BillStatusNoCharge:
		return fmt.Sprintf("➖ បន្ទប់ %d — មិនគិតលុយ", bl.RoomNumber)
	case bl.WaterCurr == nil:
		return fmt.Sprintf("⏳ បន្ទប់ %d — រង់ចាំកត់លេខម៉ែត្រ", bl.RoomNumber)
	case bl.Status == models.BillStatusPaid:
		if extra := bl.PaidUSD - bl.TotalUSD; extra > 0.01 {
			return fmt.Sprintf("✅ បន្ទប់ %d — បានបង់ (ត្រូវបង់ $%.2f, បានទទួល $%.2f, លើស $%.2f)",
				bl.RoomNumber, bl.TotalUSD, bl.PaidUSD, extra)
		}
		return fmt.Sprintf("✅ បន្ទប់ %d — បានបង់ ($%.2f)", bl.RoomNumber, bl.TotalUSD)
	case bl.Status == models.BillStatusPartial:
		return fmt.Sprintf("🟡 បន្ទប់ %d — បង់ខ្លះ ($%.2f នៃ $%.2f, នៅខ្វះ $%.2f)",
			bl.RoomNumber, bl.PaidUSD, bl.TotalUSD, bl.TotalUSD-bl.PaidUSD)
	default:
		return fmt.Sprintf("❌ បន្ទប់ %d — មិនទាន់បង់ ($%.2f)", bl.RoomNumber, bl.TotalUSD)
	}
}

func (b *Bot) sendStatusOverview(chatID int64) {
	bills, err := b.svc.AllBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "មិនទាន់មានទិន្នន័យបន្ទប់សម្រាប់ខែនេះទេ។")
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
	fmt.Fprintf(&sb, "\n%d ក្នុងចំណោម %d បន្ទប់បានបង់រួចរាល់។", paidCount, len(bills))

	b.reply(chatID, sb.String())
}

func (b *Bot) sendMissingReadingsList(chatID int64) {
	bills, err := b.svc.RoomsMissingReadings()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "✅ គ្រប់បន្ទប់បានកត់លេខម៉ែត្ររួចរាល់ហើយ។")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		label := fmt.Sprintf("បន្ទប់ %d", bl.RoomNumber)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("bill:%d", bl.RoomNumber)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "ចុចលើបន្ទប់ដើម្បីកត់លេខម៉ែត្រ៖")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) doNewMonth(chatID int64, label string) {
	if label == "" {
		msg, err := b.svc.StartNewMonthFlow(chatID)
		if err != nil {
			b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
			return
		}
		b.reply(chatID, msg)
		return
	}
	count, err := b.svc.NewMonth(label)
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	b.reply(chatID, fmt.Sprintf("📅 បានបង្កើតវិក្កយបត្រខែ %s ជាមួយចំនួន %d បន្ទប់។", label, count))
}

func (b *Bot) doSetName(chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		b.showSetNamePicker(chatID)
		return
	}
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		b.reply(chatID, "របៀបប្រើ៖ /setname <លេខបន្ទប់> <ឈ្មោះ>")
		return
	}
	roomNumber, err := strconv.Atoi(parts[0])
	if err != nil {
		b.reply(chatID, "សូមបញ្ចូលលេខបន្ទប់ជាលេខសុទ្ធ។")
		return
	}
	if err := b.svc.SetTenantName(roomNumber, parts[1]); err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	b.reply(chatID, fmt.Sprintf("បានដាក់ឈ្មោះ %q ទៅឲ្យបន្ទប់ %d។", parts[1], roomNumber))
}

func (b *Bot) doVacate(chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		b.showVacatePicker(chatID)
		return
	}
	roomNumber, err := strconv.Atoi(args)
	if err != nil {
		b.reply(chatID, "របៀបប្រើ៖ /vacate <លេខបន្ទប់>")
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
	args = strings.TrimSpace(args)
	if args == "" {
		b.showMoveInPicker(chatID)
		return
	}
	roomNumber, err := strconv.Atoi(args)
	if err != nil {
		b.reply(chatID, "របៀបប្រើ៖ /movein <លេខបន្ទប់>")
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
	if len(parts) == 0 {
		b.showPayPicker(chatID)
		return
	}
	if len(parts) != 2 {
		b.reply(chatID, "របៀបប្រើ៖ /pay <លេខបន្ទប់> <ចំនួនទឹកប្រាក់>")
		return
	}
	roomNumber, err := strconv.Atoi(parts[0])
	if err != nil {
		b.reply(chatID, "សូមបញ្ចូលលេខបន្ទប់ជាលេខសុទ្ធ។")
		return
	}
	amount, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		b.reply(chatID, "សូមបញ្ចូលចំនួនទឹកប្រាក់ជាលេខ។")
		return
	}

	bill, err := b.svc.PayRoom(roomNumber, amount)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, fmt.Sprintf("💵 បានកត់ត្រាប្រាក់ $%.2f សម្រាប់បន្ទប់ %d។\n%s", amount, roomNumber, formatBillStatus(bill)))
}

func unwrapFriendly(err error) string {
	if err == service.ErrNoActivePeriod {
		return "មិនទាន់មានវិក្កយបត្រខែនេះទេ — សូមប្រើពាក្យបញ្ជា /newmonth 2026-09 ជាមុនសិន។"
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "រកមិនឃើញបន្ទប់នេះទេ។"
	}
	return "មានបញ្ហា៖ " + err.Error()
}

func (b *Bot) reply(chatID int64, text string) {
	b.send(tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) send(msg tgbotapi.MessageConfig) {
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send error: %v", err)
	}
}

const helpText = `🤖 ជំនួយការ PTAS Bot សម្រាប់ផ្ទះជួល

/start - បើកម៉ឺនុយចម្បង ដើម្បីគ្រប់គ្រងអ្វីៗទាំងអស់បានយ៉ាងងាយស្រួល។ អ្នកក៏អាចវាយបញ្ជាផ្ទាល់បានដែរ ដូចខាងក្រោម៖

/status    - ពិនិត្យមើលរបាយការណ៍បង់ប្រាក់ប្រចាំខែ
/unpaid    - មើលបន្ទប់ដែលជំពាក់ (ចុចដើម្បីទូទាត់ប្រាក់ងាយៗ)
/pay <បន្ទប់> <លុយ> - កត់ត្រាការបង់ប្រាក់ (បង់ពេញ ឬបង់ខ្លះ)
/billing   - មើល និងបញ្ចូលលេខម៉ែត្រសម្រាប់បន្ទប់ដែលមិនទាន់មាន
/newmonth YYYY-MM - បង្កើតវិក្កយបត្រខែថ្មី
/rooms     - មើលបញ្ជីបន្ទប់ទាំងអស់របស់អ្នក
/setname <បន្ទប់> <ឈ្មោះ> - ដាក់ឈ្មោះអ្នកជួល
/vacate <បន្ទប់> - កត់ត្រាអ្នករើចេញ៖ គណនាថ្ងៃស្នាក់នៅ និងសួរលេខម៉ែត្រចុងក្រោយ
/movein <បន្ទប់> - កត់ត្រាអ្នករើចូល៖ សួរលេខម៉ែត្រថ្មី ដើម្បីចាប់ផ្តើមគិតលុយ
/cancel    - បោះបង់ប្រតិបត្តិការបច្ចុប្បន្ន

💡 នៅក្នុងគ្រុប អ្នកអាច mention ឈ្មោះខ្ញុំ ដើម្បីឆែកស្ថានភាពបន្ទប់បានយ៉ាងរហ័ស។
ឧទាហរណ៍៖ @<bot> room:4 status`
