CREATE TABLE IF NOT EXISTS payments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id    INTEGER NOT NULL REFERENCES bills(id),
    amount_usd REAL NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    paid_at    DATETIME NOT NULL
);

-- Bills already marked 'paid' by earlier imports (e.g. the 2026-08 backfill)
-- predate this table, so give each one a matching payment record — otherwise
-- the paid/owed math done from here on (partial payments, /pay) would think
-- nothing had ever been paid toward them.
INSERT INTO payments (bill_id, amount_usd, note, paid_at)
SELECT b.id, b.total_usd, 'backfilled from historical import', p.started_at
FROM bills b
JOIN billing_periods p ON p.id = b.period_id
WHERE b.status = 'paid' AND NOT EXISTS (SELECT 1 FROM payments WHERE bill_id = b.id);

UPDATE bills SET paid_at = (SELECT started_at FROM billing_periods WHERE id = bills.period_id)
WHERE status = 'paid' AND paid_at IS NULL;
