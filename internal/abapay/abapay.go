// Package abapay parses the payment notification text ABA PayWay posts to
// its own Telegram channel/group when a KHQR payment comes in, e.g.:
//
//	$66.50 paid by SIEK SOKNA (*190) on Aug 31, 06:51 PM via ABA KHQR
//	(ACLEDA Bank Plc.) at HENG SUNHOUR. Trx. ID: 178817707523501, APV: 472438.
//
// so the owner can forward one straight into the billing bot instead of
// retyping the amount by hand.
package abapay

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ErrNoMatch means text doesn't look like an ABA PayWay payment
// notification — either it's unrelated text, or ABA changed its format.
var ErrNoMatch = errors.New("text doesn't look like an ABA PayWay payment notification")

// The bank name in parentheses after the payment method is only present for
// some methods (e.g. "via ABA KHQR (ACLEDA Bank Plc.)" but plain
// "via ABA PAY at ..." with none), and the amount may or may not carry
// decimal places (e.g. "$66.50" vs "$222,000") — both are optional here.
var pattern = regexp.MustCompile(`(?i)\$\s*([\d,]+(?:\.\d{1,2})?)\s+paid by\s+(.+?)\s+\(\*?(\w+)\)\s+on\s+(.+?)\s+via\s+([^()]+?)(?:\s*\(([^()]+)\))?\s+at\s+(.+?)\.\s*trx\.?\s*id:?\s*(\d+),\s*apv:?\s*(\d+)`)

// Notification holds the fields pulled out of one ABA PayWay message.
type Notification struct {
	AmountUSD float64
	Payer     string
	Last      string // last digits of the paying card/account, e.g. "190"
	When      string // e.g. "Aug 31, 06:51 PM"
	Method    string // e.g. "ABA KHQR"
	Bank      string // e.g. "ACLEDA Bank Plc."
	Merchant  string // e.g. "HENG SUNHOUR"
	TrxID     string
	APV       string
}

// Parse extracts a Notification from raw ABA PayWay notification text.
func Parse(text string) (Notification, error) {
	m := pattern.FindStringSubmatch(text)
	if m == nil {
		return Notification{}, ErrNoMatch
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return Notification{}, ErrNoMatch
	}
	return Notification{
		AmountUSD: amount,
		Payer:     strings.TrimSpace(m[2]),
		Last:      m[3],
		When:      strings.TrimSpace(m[4]),
		Method:    strings.TrimSpace(m[5]),
		Bank:      strings.TrimSpace(m[6]),
		Merchant:  strings.TrimSpace(m[7]),
		TrxID:     m[8],
		APV:       m[9],
	}, nil
}

// Note renders a compact one-line summary suitable for storing as a
// payment's audit note — enough to cross-check against the ABA app later.
func (n Notification) Note() string {
	return fmt.Sprintf("ABA: %s (*%s) · Trx %s · APV %s · %s", n.Payer, n.Last, n.TrxID, n.APV, n.When)
}
