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

	"ptas-bot/internal/i18n"
	"ptas-bot/internal/kmnum"
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
	{Command: "start", Description: i18n.CmdStart},
	{Command: "status", Description: i18n.CmdStatus},
	{Command: "unpaid", Description: i18n.CmdUnpaid},
	{Command: "pay", Description: i18n.CmdPay},
	{Command: "billing", Description: i18n.CmdBilling},
	{Command: "rooms", Description: i18n.CmdRooms},
	{Command: "setname", Description: i18n.CmdSetName},
	{Command: "vacate", Description: i18n.CmdVacate},
	{Command: "movein", Description: i18n.CmdMoveIn},
	{Command: "newmonth", Description: i18n.CmdNewMonth},
	{Command: "total", Description: i18n.CmdTotal},
	{Command: "cancel", Description: i18n.CmdCancel},
	{Command: "help", Description: i18n.CmdHelp},
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
			b.handleMention(msg, query)
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
		b.reply(chatID, i18n.Error(err))
		return
	}
	b.reply(chatID, out)
}

func (b *Bot) handleCommand(chatID int64, cmd, args string) {
	switch cmd {
	case "start":
		b.showMainMenu(chatID)

	case "help":
		b.reply(chatID, i18n.HelpText)

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

	case "total":
		b.sendMonthlyTotal(chatID)

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
		b.reply(chatID, i18n.Cancelled)

	default:
		b.reply(chatID, i18n.UnknownCommand)
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
			b.reply(chatID, i18n.Error(err))
			return
		}
		b.reply(chatID, i18n.FullySettled+"\n"+formatBillStatus(bill))

	case strings.HasPrefix(data, "bill:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "bill:"))
		if err != nil {
			return
		}
		prompt, err := b.svc.StartReadingEntry(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, i18n.Error(err))
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
	fmt.Fprintf(sb, "%s\n", i18n.FloorDivider(floor))
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
			tgbotapi.NewInlineKeyboardButtonData(i18n.FloorDivider(floor), "noop"),
		),
	}
}

func (b *Bot) sendRoomList(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, i18n.Error(err))
		return
	}
	var sb strings.Builder
	lastFloor := 0
	for _, r := range rooms {
		floorBreaker(&sb, &lastFloor, r.Floor)
		status := i18n.Occupied
		if r.IsVacant {
			status = i18n.Vacant
		}
		fmt.Fprintf(&sb, "%s\n", i18n.RoomLine(r.Number, status, r.BaseRentUSD))
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
		b.reply(chatID, i18n.EveryonePaid)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		amount := bl.TotalUSD - bl.PaidUSD
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				i18n.UnpaidButtonLabel(bl.RoomNumber, amount),
				fmt.Sprintf("paid:%d", bl.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, i18n.TapToSettle)
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
		return i18n.StatusVacant(bl.RoomNumber)
	case bl.Status == models.BillStatusNoCharge:
		return i18n.StatusNoCharge(bl.RoomNumber)
	case bl.WaterCurr == nil:
		return i18n.StatusNoReadings(bl.RoomNumber)
	case bl.Status == models.BillStatusPaid:
		if extra := bl.PaidUSD - bl.TotalUSD; extra > 0.01 {
			return i18n.StatusPaidWithOverpay(bl.RoomNumber, bl.TotalUSD, bl.PaidUSD, extra)
		}
		return i18n.StatusPaid(bl.RoomNumber, bl.TotalUSD)
	case bl.Status == models.BillStatusPartial:
		return i18n.StatusPartial(bl.RoomNumber, bl.PaidUSD, bl.TotalUSD, bl.TotalUSD-bl.PaidUSD)
	default:
		return i18n.StatusUnpaid(bl.RoomNumber, bl.TotalUSD)
	}
}

func (b *Bot) sendStatusOverview(chatID int64) {
	bills, err := b.svc.AllBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, i18n.NoRoomsThisMonth)
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
	fmt.Fprintf(&sb, "\n%s", i18n.PaidCount(paidCount, len(bills)))

	b.reply(chatID, sb.String())
}

// sendMonthlyTotal shows the current month's billed/collected/outstanding
// totals across every room — the dollar-figure counterpart to /status's
// per-room breakdown.
func (b *Bot) sendMonthlyTotal(chatID int64) {
	billed, collected, err := b.svc.MonthlyTotals()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, i18n.MonthlyTotal(billed, collected))
}

func (b *Bot) sendMissingReadingsList(chatID int64) {
	bills, err := b.svc.RoomsMissingReadings()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, i18n.AllReadingsEntered)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		label := i18n.RoomLabel(bl.RoomNumber)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("bill:%d", bl.RoomNumber)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, i18n.TapToEnterReadings)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) doNewMonth(chatID int64, label string) {
	label = kmnum.ToArabic(label)
	if label == "" {
		msg, err := b.svc.StartNewMonthFlow(chatID)
		if err != nil {
			b.reply(chatID, i18n.Error(err))
			return
		}
		b.reply(chatID, msg)
		return
	}
	count, err := b.svc.NewMonth(label)
	if err != nil {
		b.reply(chatID, i18n.Error(err))
		return
	}
	b.reply(chatID, i18n.MonthOpened(label, count))
}

func (b *Bot) doSetName(chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		b.showSetNamePicker(chatID)
		return
	}
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		b.reply(chatID, i18n.UsageSetName)
		return
	}
	roomNumber, err := strconv.Atoi(kmnum.ToArabic(parts[0]))
	if err != nil {
		b.reply(chatID, i18n.RoomNumberMustBeInt)
		return
	}
	if err := b.svc.SetTenantName(roomNumber, parts[1]); err != nil {
		b.reply(chatID, i18n.Error(err))
		return
	}
	b.reply(chatID, i18n.SetNameDone(roomNumber, parts[1]))
}

func (b *Bot) doVacate(chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		b.showVacatePicker(chatID)
		return
	}
	roomNumber, err := strconv.Atoi(kmnum.ToArabic(args))
	if err != nil {
		b.reply(chatID, i18n.UsageVacate)
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
	roomNumber, err := strconv.Atoi(kmnum.ToArabic(args))
	if err != nil {
		b.reply(chatID, i18n.UsageMoveIn)
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
		b.reply(chatID, i18n.UsagePay)
		return
	}
	roomNumber, err := strconv.Atoi(kmnum.ToArabic(parts[0]))
	if err != nil {
		b.reply(chatID, i18n.RoomNumberMustBeInt)
		return
	}
	amount, err := strconv.ParseFloat(kmnum.ToArabic(parts[1]), 64)
	if err != nil {
		b.reply(chatID, i18n.AmountMustBeNumber)
		return
	}

	bill, err := b.svc.PayRoom(roomNumber, amount)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, i18n.RecordedPayment(amount, roomNumber)+"\n"+formatBillStatus(bill))
}

func unwrapFriendly(err error) string {
	if err == service.ErrNoActivePeriod {
		return i18n.NoBillingMonth
	}
	if errors.Is(err, sql.ErrNoRows) {
		return i18n.RoomNotFound
	}
	if err == service.ErrDuplicateTransaction {
		return i18n.DuplicateTransaction
	}
	return i18n.ErrorPlain(err)
}

func (b *Bot) reply(chatID int64, text string) {
	b.send(tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) send(msg tgbotapi.MessageConfig) {
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send error: %v", err)
	}
}
