package telegram

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMenuCallback routes every "menu:*" and "flow:*" callback — the
// BotFather-style nested menu system: /start's main menu drills into a room
// picker, which drills into a per-room action submenu, and the same action
// buttons appear whether you got there via the picker or the submenu.
func (b *Bot) handleMenuCallback(chatID int64, data string) bool {
	switch {
	case data == "menu:main":
		b.showMainMenu(chatID)
	case data == "menu:rooms":
		b.showRoomPicker(chatID)
	case strings.HasPrefix(data, "menu:room:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "menu:room:"))
		if err != nil {
			return true
		}
		b.showRoomSubmenu(chatID, roomNumber)
	case data == "menu:status":
		b.sendStatusOverview(chatID)
	case data == "menu:unpaid":
		b.sendUnpaidList(chatID)
	case data == "menu:billing":
		b.sendMissingReadingsList(chatID)
	case data == "menu:pay":
		b.showPayPicker(chatID)
	case data == "menu:setname":
		b.showSetNamePicker(chatID)
	case data == "menu:vacate":
		b.showVacatePicker(chatID)
	case data == "menu:movein":
		b.showMoveInPicker(chatID)
	case data == "menu:newmonth":
		msg, err := b.svc.StartNewMonthFlow(chatID)
		if err != nil {
			b.reply(chatID, "Error: "+err.Error())
			return true
		}
		b.reply(chatID, msg)

	case strings.HasPrefix(data, "flow:pay:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "flow:pay:"))
		if err != nil {
			return true
		}
		msg, err := b.svc.StartPayFlow(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, unwrapFriendly(err))
			return true
		}
		b.reply(chatID, msg)

	case strings.HasPrefix(data, "flow:setname:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "flow:setname:"))
		if err != nil {
			return true
		}
		msg, err := b.svc.StartSetNameFlow(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, unwrapFriendly(err))
			return true
		}
		b.reply(chatID, msg)

	case strings.HasPrefix(data, "flow:vacate:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "flow:vacate:"))
		if err != nil {
			return true
		}
		msg, err := b.svc.StartVacateEntry(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, unwrapFriendly(err))
			return true
		}
		b.reply(chatID, msg)

	case strings.HasPrefix(data, "flow:movein:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "flow:movein:"))
		if err != nil {
			return true
		}
		msg, err := b.svc.StartMoveIn(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, unwrapFriendly(err))
			return true
		}
		b.reply(chatID, msg)

	default:
		return false
	}
	return true
}

func (b *Bot) showMainMenu(chatID int64) {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Status", "menu:status"),
			tgbotapi.NewInlineKeyboardButtonData("💰 Unpaid", "menu:unpaid"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Billing", "menu:billing"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Rooms", "menu:rooms"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Pay", "menu:pay"),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Set Name", "menu:setname"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 Vacate", "menu:vacate"),
			tgbotapi.NewInlineKeyboardButtonData("🔑 Move In", "menu:movein"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 New Month", "menu:newmonth"),
		),
	}
	msg := tgbotapi.NewMessage(chatID, "PTAS billing bot — what do you need?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// showRoomPicker lists every room, grouped by floor; tapping one opens its
// action submenu.
func (b *Bot) showRoomPicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("Room %d", r.Number)
		if r.IsVacant {
			label += " 🚪"
		} else if r.TenantName != "" {
			label += " — " + r.TenantName
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("menu:room:%d", r.Number)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Main menu", "menu:main"),
	))

	msg := tgbotapi.NewMessage(chatID, "Pick a room:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// showRoomSubmenu shows the current status line for one room plus every
// action that can be taken on it — the nested "drill down" screen.
func (b *Bot) showRoomSubmenu(chatID int64, roomNumber int) {
	var status string
	if bill, err := b.svc.RoomStatus(roomNumber); err == nil {
		status = formatBillStatus(bill)
	} else {
		status = fmt.Sprintf("Room %d", roomNumber)
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Pay", fmt.Sprintf("flow:pay:%d", roomNumber)),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Set Name", fmt.Sprintf("flow:setname:%d", roomNumber)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 Vacate", fmt.Sprintf("flow:vacate:%d", roomNumber)),
			tgbotapi.NewInlineKeyboardButtonData("🔑 Move In", fmt.Sprintf("flow:movein:%d", roomNumber)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ All rooms", "menu:rooms"),
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Main menu", "menu:main"),
		),
	}
	msg := tgbotapi.NewMessage(chatID, status)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showPayPicker(chatID int64) {
	bills, err := b.svc.UnpaidBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "🎉 Nobody owes anything this period.")
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		amount := bl.TotalUSD - bl.PaidUSD
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("Room %d — owes $%.2f", bl.RoomNumber, amount),
				fmt.Sprintf("flow:pay:%d", bl.RoomNumber)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Who paid?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showSetNamePicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("Room %d", r.Number)
		if r.TenantName != "" {
			label += " — " + r.TenantName
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("flow:setname:%d", r.Number)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "Set the name for which room?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showVacatePicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		if r.IsVacant {
			continue
		}
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("Room %d", r.Number)
		if r.TenantName != "" {
			label += " — " + r.TenantName
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("flow:vacate:%d", r.Number)),
		))
	}
	if len(rows) == 0 {
		b.reply(chatID, "Every room is already vacant.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Which room is moving out?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showMoveInPicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "Error: "+err.Error())
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		if !r.IsVacant {
			continue
		}
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Room %d", r.Number), fmt.Sprintf("flow:movein:%d", r.Number)),
		))
	}
	if len(rows) == 0 {
		b.reply(chatID, "No vacant rooms right now.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Which room is a new tenant moving into?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}
