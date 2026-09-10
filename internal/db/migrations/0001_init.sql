CREATE TABLE IF NOT EXISTS rooms (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    number        INTEGER NOT NULL UNIQUE,
    floor         INTEGER NOT NULL,
    tenant_name   TEXT NOT NULL DEFAULT '',
    base_rent_usd REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS billing_periods (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    label      TEXT NOT NULL UNIQUE,
    started_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS bills (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id          INTEGER NOT NULL REFERENCES rooms(id),
    period_id        INTEGER NOT NULL REFERENCES billing_periods(id),
    days_stayed      INTEGER NOT NULL DEFAULT 31,
    rent_usd         REAL NOT NULL DEFAULT 0,
    water_prev       REAL NOT NULL DEFAULT 0,
    water_curr       REAL,
    water_used       REAL NOT NULL DEFAULT 0,
    water_cost_riel  REAL NOT NULL DEFAULT 0,
    elec_prev        REAL NOT NULL DEFAULT 0,
    elec_curr        REAL,
    elec_used        REAL NOT NULL DEFAULT 0,
    elec_cost_riel   REAL NOT NULL DEFAULT 0,
    extra_charge_usd REAL NOT NULL DEFAULT 0,
    total_riel       REAL NOT NULL DEFAULT 0,
    total_usd        REAL NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'unpaid',
    notes            TEXT NOT NULL DEFAULT '',
    paid_at          DATETIME,
    UNIQUE (room_id, period_id)
);

CREATE TABLE IF NOT EXISTS pending_actions (
    chat_id    INTEGER PRIMARY KEY,
    kind       TEXT NOT NULL,
    bill_id    INTEGER NOT NULL REFERENCES bills(id),
    step       TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS payment_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    raw_text        TEXT NOT NULL,
    amount_usd      REAL NOT NULL,
    payer_name      TEXT NOT NULL,
    trx_id          TEXT NOT NULL DEFAULT '',
    apv             TEXT NOT NULL DEFAULT '',
    occurred_at     DATETIME NOT NULL,
    matched_bill_id INTEGER REFERENCES bills(id)
);
