-- An entry can have one reminder. remind_at is wall-clock time with no zone,
-- so 08:00 stays 08:00 wherever the phone is; remind_rule is an optional
-- RFC 5545 RRULE that repeats it. Phones schedule the alarms; the server
-- only stores them.
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS remind_at TIMESTAMP;
ALTER TABLE lists.items ADD COLUMN IF NOT EXISTS remind_rule TEXT;
