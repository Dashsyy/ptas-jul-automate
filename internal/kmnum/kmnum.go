// Package kmnum normalizes Khmer numerals to Arabic ones. Khmer keyboards
// commonly produce ០-៩ instead of 0-9, but every numeric input in this app
// (meter readings, payment amounts, room numbers, month labels) is parsed
// with the standard library's strconv, which only understands Arabic
// digits — so every such input is normalized through ToArabic first.
package kmnum

import "strings"

// ToArabic rewrites Khmer digits (០-៩, U+17E0-U+17E9) within s to their
// Arabic equivalents. Everything else — including regular Arabic digits —
// passes through unchanged, so it's safe to call on any user-typed text
// before handing it to strconv.
func ToArabic(s string) string {
	const khmerZero = '០'
	hasKhmerDigit := false
	for _, r := range s {
		if r >= khmerZero && r <= '៩' {
			hasKhmerDigit = true
			break
		}
	}
	if !hasKhmerDigit {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= khmerZero && r <= '៩' {
			b.WriteRune('0' + (r - khmerZero))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
