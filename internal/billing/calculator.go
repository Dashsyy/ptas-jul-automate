package billing

import "time"

type Rates struct {
	WaterRiel float64 // per m3
	ElecRiel  float64 // per kWh
	USDRiel   float64 // riel per 1 USD
}

type ReadingsInput struct {
	RentUSD     float64
	DaysStayed  int
	DaysInMonth int
	WaterPrev   float64
	WaterCurr   float64
	ElecPrev    float64
	ElecCurr    float64
}

type Computed struct {
	RentRiel      float64
	WaterUsed     float64
	WaterCostRiel float64
	ElecUsed      float64
	ElecCostRiel  float64
	TotalRiel     float64
	TotalUSD      float64
}

// Compute mirrors the spreadsheet's math: rent is prorated by days stayed
// over days in the month, then converted to Riel; water/electricity cost is
// usage times the fixed per-unit rate; everything is summed in Riel and also
// expressed in USD using the configured exchange rate.
func Compute(in ReadingsInput, rates Rates) Computed {
	daysInMonth := in.DaysInMonth
	if daysInMonth <= 0 {
		daysInMonth = 31
	}
	daysStayed := in.DaysStayed
	if daysStayed <= 0 {
		daysStayed = daysInMonth
	}

	rentRiel := in.RentUSD * rates.USDRiel * float64(daysStayed) / float64(daysInMonth)

	waterUsed := in.WaterCurr - in.WaterPrev
	if waterUsed < 0 {
		waterUsed = 0
	}
	waterCost := waterUsed * rates.WaterRiel

	elecUsed := in.ElecCurr - in.ElecPrev
	if elecUsed < 0 {
		elecUsed = 0
	}
	elecCost := elecUsed * rates.ElecRiel

	totalRiel := rentRiel + waterCost + elecCost
	totalUSD := totalRiel / rates.USDRiel

	return Computed{
		RentRiel:      rentRiel,
		WaterUsed:     waterUsed,
		WaterCostRiel: waterCost,
		ElecUsed:      elecUsed,
		ElecCostRiel:  elecCost,
		TotalRiel:     totalRiel,
		TotalUSD:      totalUSD,
	}
}

// DaysInMonth returns the number of days in the month containing t.
func DaysInMonth(t time.Time) int {
	firstOfNext := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	return firstOfNext.AddDate(0, 0, -1).Day()
}
