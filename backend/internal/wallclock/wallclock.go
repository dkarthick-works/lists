// Package wallclock is a date and time with no time zone, as stored in a
// Postgres TIMESTAMP column: 08:00 means 08:00 wherever the reader is.
package wallclock

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Layout is the JSON form, to the minute and with no zone: "2026-10-17T08:00".
const Layout = "2006-01-02T15:04"

// Time embeds pgtype.Timestamp, so pgx scans and encodes it as a TIMESTAMP.
// The zero value is SQL NULL and JSON null.
type Time struct{ pgtype.Timestamp }

func Parse(s string) (Time, error) {
	t, err := time.Parse(Layout, s)
	if err != nil {
		return Time{}, err
	}
	return Time{pgtype.Timestamp{Time: t, Valid: true}}, nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.Format(Layout))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var s *string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == nil {
		*t = Time{}
		return nil
	}
	p, err := Parse(*s)
	if err != nil {
		return err
	}
	*t = p
	return nil
}
