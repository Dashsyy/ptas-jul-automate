-- Seeds the 14 rooms with number/floor/rent from the existing spreadsheet.
-- Everything is keyed by room number; tenant_name is optional/cosmetic and
-- left blank here — set it later with /setname <room> <name> if you want it
-- to show up in messages, but nothing depends on it.
INSERT OR IGNORE INTO rooms (number, floor, tenant_name, base_rent_usd) VALUES
    (1, 1, '', 60),
    (2, 1, '', 60),
    (3, 1, '', 60),
    (4, 1, '', 60),
    (5, 1, '', 60),
    (6, 1, '', 60),
    (7, 1, '', 60),
    (8, 2, '', 50),
    (9, 2, '', 50),
    (10, 2, '', 50),
    (11, 2, '', 50),
    (12, 2, '', 50),
    (13, 2, '', 50),
    (14, 2, '', 50);
