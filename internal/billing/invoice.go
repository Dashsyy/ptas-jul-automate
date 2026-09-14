package billing

import (
	"fmt"
	"strconv"
	"strings"

	"ptas-bot/internal/i18n"
)

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

	fmt.Fprintf(&b, "%s\n", i18n.InvoiceTitle)
	fmt.Fprintf(&b, "%s\n\n", i18n.InvoiceMonthLine(in.PeriodLabel))
	fmt.Fprintf(&b, "%s\n\n", i18n.InvoiceRoomLine(in.RoomNumber))

	fmt.Fprintf(&b, "%s\n", i18n.InvoiceTableHeader)
	fmt.Fprintf(&b, "%s      %-8s %-8s %-6s %s ៛\n",
		i18n.InvoiceRowWater, fmtNum(in.WaterCurr), fmtNum(in.WaterPrev), fmtNum(in.WaterUsed), fmtRiel(in.WaterCost))
	fmt.Fprintf(&b, "%s     %-8s %-8s %-6s %s ៛\n\n",
		i18n.InvoiceRowElec, fmtNum(in.ElecCurr), fmtNum(in.ElecPrev), fmtNum(in.ElecUsed), fmtRiel(in.ElecCost))

	fmt.Fprintf(&b, "%s\n\n", i18n.InvoiceRoomRentLine(fmtNum(in.RentUSD), in.DaysStayed, in.DaysInMonth, fmtRiel(in.RentRiel)))

	fmt.Fprintf(&b, "%s", i18n.InvoiceTotalLine(fmtRiel(in.TotalRiel), fmtNum(in.TotalUSD)))

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
