-- Backfills the August 2026 billing period from the existing paper/sheet
-- records (docs/IMG_6537.JPG) so the app has real history from day one, and
-- so /newmonth 2026-09 carries the correct "previous" meter readings forward
-- instead of starting every room at 0.
--
-- Every riel total below was independently recomputed from
-- rent*days_stayed/31 + water_used*2500 + elec_used*1500 and cross-checked
-- against the sheet's floor subtotals (1,959,000 / 1,742,339 / 3,701,339 ៛) —
-- all matched exactly, so the readings/days-stayed are trustworthy. Two
-- judgment calls could not be resolved from the sheet and are flagged in
-- notes for manual review:
--   - Room 5's Payment Status cell was greyed out/ambiguous (tenant marked
--     "leave" mid-sheet) -> imported as 'unpaid', flagged for review.
--   - Room 7 was explicitly "No charge" (cost tracked only, tenant not
--     billed) -> imported as 'no_charge'.
INSERT OR IGNORE INTO billing_periods (label, started_at) VALUES ('2026-08', '2026-08-01 00:00:00');

-- SQLite's table-valued VALUES() clause can't take a column-alias list (that's
-- Postgres syntax), so columns are referenced by their default names
-- (column1, column2, ...) in the order listed below:
--   1 room_number, 2 days_stayed, 3 water_prev, 4 water_curr, 5 water_used,
--   6 water_cost_riel, 7 elec_prev, 8 elec_curr, 9 elec_used, 10 elec_cost_riel,
--   11 total_riel, 12 total_usd, 13 status, 14 notes
INSERT OR IGNORE INTO bills (
    room_id, period_id, days_stayed, rent_usd,
    water_prev, water_curr, water_used, water_cost_riel,
    elec_prev, elec_curr, elec_used, elec_cost_riel,
    total_riel, total_usd, status, notes
)
SELECT r.id, p.id, v.column2, r.base_rent_usd,
       v.column3, v.column4, v.column5, v.column6,
       v.column7, v.column8, v.column9, v.column10,
       v.column11, v.column12, v.column13, v.column14
FROM (VALUES
    (1,  31, 5,  7,  2,  5000.0,  22, 36,  14, 21000.0, 266000.0, 66.50, 'paid',   ''),
    (2,  31, 14, 22, 8,  20000.0, 34, 52,  18, 27000.0, 287000.0, 71.75, 'paid',   ''),
    (3,  31, 9,  13, 4,  10000.0, 35, 47,  12, 18000.0, 268000.0, 67.00, 'unpaid', ''),
    (4,  31, 8,  14, 6,  15000.0, 31, 57,  26, 39000.0, 294000.0, 73.50, 'paid',   '54000r (Leave)'),
    (5,  31, 8,  12, 4,  10000.0, 31, 46,  15, 22500.0, 272500.0, 68.13, 'unpaid', 'Row greyed out in original sheet (tenant marked leave) — verify payment status manually'),
    (6,  31, 15, 22, 7,  17500.0, 49, 67,  18, 27000.0, 284500.0, 71.13, 'paid',   '49000r (Leave)'),
    (7,  31, 3,  5,  2,  5000.0,  31, 59,  28, 42000.0, 287000.0, 71.75, 'no_charge', 'Does not pay — cost tracked only'),
    (8,  31, 7,  12, 5,  12500.0, 40, 72,  32, 48000.0, 260500.0, 65.13, 'unpaid', ''),
    (9,  31, 9,  15, 6,  15000.0, 41, 66,  25, 37500.0, 252500.0, 63.13, 'unpaid', ''),
    (10, 24, 8,  20, 12, 30000.0, 41, 55,  14, 21000.0, 205839.0, 51.46, 'paid',   '51000r (Leave)'),
    (11, 31, 2,  3,  1,  2500.0,  13, 26,  13, 19500.0, 222000.0, 55.50, 'paid',   ''),
    (12, 31, 3,  7,  4,  10000.0, 13, 29,  16, 24000.0, 234000.0, 58.50, 'unpaid', ''),
    (13, 31, 10, 17, 7,  17500.0, 99, 151, 52, 78000.0, 295500.0, 73.88, 'unpaid', ''),
    (14, 31, 17, 26, 9,  22500.0, 50, 83,  33, 49500.0, 272000.0, 68.00, 'unpaid', '')
) AS v
JOIN rooms r ON r.number = v.column1
JOIN billing_periods p ON p.label = '2026-08';
