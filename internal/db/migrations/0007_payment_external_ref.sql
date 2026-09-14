-- external_ref holds a source-system identifier (e.g. an ABA PayWay
-- transaction ID) for payments imported via /update_info, so the same
-- forwarded notification can't be recorded twice by accident. Manual
-- payments (typed amounts, "Pay full") leave this blank.
ALTER TABLE payments ADD COLUMN external_ref TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_payments_external_ref ON payments(external_ref);
