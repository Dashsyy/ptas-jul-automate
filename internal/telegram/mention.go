package telegram

import (
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ptas-bot/internal/abapay"
	"ptas-bot/internal/i18n"
	"ptas-bot/internal/kmnum"
)

// roomQueryRe matches a room number in either Arabic (0-9) or Khmer (០-៩)
// digits — Khmer keyboards commonly substitute Khmer numerals for typed
// Arabic ones, so a query like "room:៥ status" must parse the same as
// "room:5 status".
var roomQueryRe = regexp.MustCompile(`(?i)room\s*[:#]?\s*([0-9\x{17E0}-\x{17E9}]+)`)

// updateInfoRe recognizes the "update_info" mention sub-command.
var updateInfoRe = regexp.MustCompile(`(?i)^update_info\b`)

// mentionQuery reports whether the bot is @mentioned in msg and, if so,
// returns the text following the mention. Telegram delivers @mentions to a
// bot regardless of its group privacy setting, so this works without the
// bot needing to see every message in the group.
func (b *Bot) mentionQuery(msg *tgbotapi.Message) (string, bool) {
	if b.username == "" || msg.Text == "" {
		return "", false
	}
	mention := "@" + strings.ToLower(b.username)
	idx := strings.Index(strings.ToLower(msg.Text), mention)
	if idx == -1 {
		return "", false
	}
	return strings.TrimSpace(msg.Text[idx+len(mention):]), true
}

// handleMention answers whatever the bot was @mentioned to do: a
// "room:<n> status"-style read-only query, or an "update_info room <n>"
// payment import. It's the only thing the bot acts on outside a private
// DM, and only from the owner.
func (b *Bot) handleMention(msg *tgbotapi.Message, query string) {
	if updateInfoRe.MatchString(query) {
		b.updateInfoFromReply(msg.Chat.ID, msg.ReplyToMessage, query)
		return
	}

	chatID := msg.Chat.ID
	m := roomQueryRe.FindStringSubmatch(query)
	if m == nil {
		b.reply(chatID, i18n.MentionUsageHint(b.username))
		return
	}

	roomNumber, err := strconv.Atoi(kmnum.ToArabic(m[1]))
	if err != nil {
		return
	}
	bill, err := b.svc.RoomStatus(roomNumber)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, formatBillStatus(bill))
}

// updateInfoFromReply records a payment by parsing the ABA PayWay
// notification replyTo holds. The intended flow: forward the ABA PayWay
// notification into the chat, then reply to it with either
// "/update_info room 4" (in a private DM) or "@<bot> update_info room 4"
// (in a group) — the bot reads the forwarded message's amount and payer,
// and records the payment against room 4 with the payer's name and
// transaction ID kept as the payment's note. argsText is whatever followed
// the command/mention — just needs to contain a "room <n>" somewhere.
func (b *Bot) updateInfoFromReply(chatID int64, replyTo *tgbotapi.Message, argsText string) {
	m := roomQueryRe.FindStringSubmatch(argsText)
	if m == nil {
		b.reply(chatID, i18n.UpdateInfoUsageHint(b.username))
		return
	}
	roomNumber, err := strconv.Atoi(kmnum.ToArabic(m[1]))
	if err != nil {
		b.reply(chatID, i18n.UpdateInfoUsageHint(b.username))
		return
	}

	if replyTo == nil || replyTo.Text == "" {
		b.reply(chatID, i18n.UpdateInfoNeedsReply)
		return
	}

	notif, err := abapay.Parse(replyTo.Text)
	if err != nil {
		b.reply(chatID, i18n.UpdateInfoParseFailed)
		return
	}

	bill, err := b.svc.PayRoomFromNotification(roomNumber, notif.AmountUSD, notif.Note(), notif.TrxID)
	if err != nil {
		b.reply(chatID, unwrapFriendly(err))
		return
	}
	b.reply(chatID, i18n.UpdateInfoRecorded(notif.Payer, notif.AmountUSD, roomNumber)+"\n"+formatBillStatus(bill))
}
