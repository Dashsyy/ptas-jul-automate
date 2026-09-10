package telegram

import (
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var roomQueryRe = regexp.MustCompile(`(?i)room\s*[:#]?\s*(\d+)`)

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

// handleMention answers a "room:<n> status"-style query sent by @mentioning
// the bot in a group (e.g. the ABA PayWay notifications group). It's the
// only thing the bot acts on outside a private DM, and only from the owner.
func (b *Bot) handleMention(chatID int64, query string) {
	m := roomQueryRe.FindStringSubmatch(query)
	if m == nil {
		b.reply(chatID, "Try: @"+b.username+" room:4 status")
		return
	}

	roomNumber, err := strconv.Atoi(m[1])
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
