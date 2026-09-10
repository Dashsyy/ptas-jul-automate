package billing

import (
	"fmt"
	"strconv"
	"strings"
)

var khmerMonths = [...]string{
	"មករា", "កុម្ភៈ", "មីនា", "មេសា", "ឧសភា", "មិថុនា",
	"កក្កដា", "សីហា", "កញ្ញា", "តុលា", "វិច្ឆិកា", "ធ្នូ",
}

// KhmerMonthYear renders month (1-12) and year in Khmer, e.g. "សីហា ២០២៦".
func KhmerMonthYear(month int, year int) string {
	if month < 1 || month > 12 {
		return fmt.Sprintf("%d/%d", month, year)
	}
	return fmt.Sprintf("%s %d", khmerMonths[month-1], year)
}

type InvoiceInput struct {
	PeriodLabel string // human label, e.g. "សីហា 2026"
	RoomNumber  int
	DaysStayed  int
	DaysInMonth int
	RentUSD     float64
	WaterPrev   float64
	WaterCurr   float64
	WaterUsed   float64
	WaterCost   float64
	ElecPrev    float64
	ElecCurr    float64
	ElecUsed    float64
	ElecCost    float64
	RentRiel    float64
	TotalRiel   float64
	TotalUSD    float64
}

// RenderKhmerInvoice produces the per-room tenant receipt text, matching the
// layout of the property's existing paper invoice.
func RenderKhmerInvoice(in InvoiceInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "វិក្កយបត្រ បន្ទប់ជួល\n")
	fmt.Fprintf(&b, "ខែ %s\n\n", in.PeriodLabel)
	fmt.Fprintf(&b, "បន្ទប់លេខ %d\n\n", in.RoomNumber)

	fmt.Fprintf(&b, "         លេខថ្មី   លេខចាស់   ចំនួន   សរុប\n")
	fmt.Fprintf(&b, "ទឹក      %-8s %-8s %-6s %s ៛\n",
		fmtNum(in.WaterCurr), fmtNum(in.WaterPrev), fmtNum(in.WaterUsed), fmtRiel(in.WaterCost))
	fmt.Fprintf(&b, "ភ្លើង     %-8s %-8s %-6s %s ៛\n\n",
		fmtNum(in.ElecCurr), fmtNum(in.ElecPrev), fmtNum(in.ElecUsed), fmtRiel(in.ElecCost))

	fmt.Fprintf(&b, "ថ្លៃបន្ទប់: $%s × %d/%d ថ្ងៃ = %s ៛ ($%s)\n\n",
		fmtNum(in.RentUSD), in.DaysStayed, in.DaysInMonth, fmtRiel(in.RentRiel), fmtNum(in.RentUSD))

	fmt.Fprintf(&b, "សរុបទឹកប្រាក់: %s ៛  /  $%s", fmtRiel(in.TotalRiel), fmtNum(in.TotalUSD))

	return b.String()
}

func fmtNum(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// fmtRiel adds thousands separators, e.g. 284500 -> "284,500".
func fmtRiel(v float64) string {
	s := strconv.FormatInt(int64(v+0.5), 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
