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
			b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
			return true
		}
		b.reply(chatID, msg)

	case strings.HasPrefix(data, "flow:payfull:"):
		roomNumber, err := strconv.Atoi(strings.TrimPrefix(data, "flow:payfull:"))
		if err != nil {
			return true
		}
		bill, err := b.svc.PayFull(chatID, roomNumber)
		if err != nil {
			b.reply(chatID, unwrapFriendly(err))
			return true
		}
		b.reply(chatID, "✅ ទូទាត់រួចរាល់។\n"+formatBillStatus(bill))

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
		b.sendPayPrompt(chatID, roomNumber, msg)

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
			tgbotapi.NewInlineKeyboardButtonData("📊 ស្ថានភាព", "menu:status"),
			tgbotapi.NewInlineKeyboardButtonData("💰 នៅជំពាក់", "menu:unpaid"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 គិតលុយ", "menu:billing"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 បន្ទប់", "menu:rooms"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 បង់ប្រាក់", "menu:pay"),
			tgbotapi.NewInlineKeyboardButtonData("✏️ ដាក់ឈ្មោះ", "menu:setname"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 រើចេញ", "menu:vacate"),
			tgbotapi.NewInlineKeyboardButtonData("🔑 រើចូល", "menu:movein"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 ខែថ្មី", "menu:newmonth"),
		),
	}
	msg := tgbotapi.NewMessage(chatID, "ជំរាបសួរពី PTAS Bot! 👋 តើថ្ងៃនេះមានអ្វីឲ្យខ្ញុំជួយ?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// showRoomPicker lists every room, grouped by floor; tapping one opens its
// action submenu.
func (b *Bot) showRoomPicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("បន្ទប់ទី %d", r.Number)
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
		tgbotapi.NewInlineKeyboardButtonData("⬅️ ត្រឡប់ទៅម៉ឺនុយដើម", "menu:main"),
	))

	msg := tgbotapi.NewMessage(chatID, "សូមជ្រើសរើសបន្ទប់៖")
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
		status = fmt.Sprintf("បន្ទប់ទី %d", roomNumber)
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 បង់ប្រាក់", fmt.Sprintf("flow:pay:%d", roomNumber)),
			tgbotapi.NewInlineKeyboardButtonData("✏️ ដាក់ឈ្មោះ", fmt.Sprintf("flow:setname:%d", roomNumber)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 រើចេញ", fmt.Sprintf("flow:vacate:%d", roomNumber)),
			tgbotapi.NewInlineKeyboardButtonData("🔑 រើចូល", fmt.Sprintf("flow:movein:%d", roomNumber)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ ត្រឡប់ទៅបន្ទប់ទាំងអស់", "menu:rooms"),
			tgbotapi.NewInlineKeyboardButtonData("⬅️ ត្រឡប់ទៅម៉ឺនុយដើម", "menu:main"),
		),
	}
	msg := tgbotapi.NewMessage(chatID, status)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// sendPayPrompt shows the "how much did they pay?" prompt with a "Pay full"
// quick-reply button attached — most tenants pay in full, so this saves them
// typing the exact amount, while the pending "pay_amount" action set by
// StartPayFlow still accepts a typed number for the rarer partial payment.
func (b *Bot) sendPayPrompt(chatID int64, roomNumber int, prompt string) {
	msg := tgbotapi.NewMessage(chatID, prompt)
	if bill, err := b.svc.RoomStatus(roomNumber); err == nil {
		if remaining := bill.TotalUSD - bill.PaidUSD; remaining > 0.01 {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("✅ បង់ពេញ ($%.2f)", remaining),
						fmt.Sprintf("flow:payfull:%d", roomNumber)),
				),
			)
		}
	}
	b.send(msg)
}

func (b *Bot) showPayPicker(chatID int64) {
	bills, err := b.svc.UnpaidBills()
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	if len(bills) == 0 {
		b.reply(chatID, "🎉 អបអរសាទរ! គ្មានបន្ទប់ណាជំពាក់លុយទេខែនេះ។")
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, bl := range bills {
		rows = append(rows, floorBreakerRow(&lastFloor, bl.RoomFloor)...)
		amount := bl.TotalUSD - bl.PaidUSD
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("បន្ទប់ %d — ជំពាក់ $%.2f", bl.RoomNumber, amount),
				fmt.Sprintf("flow:pay:%d", bl.RoomNumber)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "តើបន្ទប់លេខប៉ុន្មានដែលបានបង់លុយ?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showSetNamePicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("បន្ទប់ទី %d", r.Number)
		if r.TenantName != "" {
			label += " — " + r.TenantName
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("flow:setname:%d", r.Number)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "តើអ្នកចង់ដាក់ឈ្មោះឲ្យបន្ទប់មួយណា?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showVacatePicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	lastFloor := 0
	for _, r := range rooms {
		if r.IsVacant {
			continue
		}
		rows = append(rows, floorBreakerRow(&lastFloor, r.Floor)...)
		label := fmt.Sprintf("បន្ទប់ទី %d", r.Number)
		if r.TenantName != "" {
			label += " — " + r.TenantName
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("flow:vacate:%d", r.Number)),
		))
	}
	if len(rows) == 0 {
		b.reply(chatID, "គ្រប់បន្ទប់ទំនេរអស់ហើយពេលនេះ។")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "តើអ្នកជួលបន្ទប់មួយណាកំពុងរើចេញ?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

func (b *Bot) showMoveInPicker(chatID int64) {
	rooms, err := b.svc.ListRooms()
	if err != nil {
		b.reply(chatID, "⚠️ មានបញ្ហាបន្តិច៖ "+err.Error())
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
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("បន្ទប់ទី %d", r.Number), fmt.Sprintf("flow:movein:%d", r.Number)),
		))
	}
	if len(rows) == 0 {
		b.reply(chatID, "សុំទោស គ្មានបន្ទប់ទំនេរទេពេលនេះ។")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "តើអ្នកជួលថ្មីនឹងស្នាក់នៅបន្ទប់មួយណា?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}
