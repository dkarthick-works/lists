package api

import (
	"encoding/json"
	"testing"

	"lists/internal/wallclock"
)

func TestCheckRule(t *testing.T) {
	for raw, want := range map[string]string{
		"FREQ=DAILY":                        "FREQ=DAILY",
		"rrule:freq=weekly;byday=mo,th":     "FREQ=WEEKLY;BYDAY=MO,TH",
		"FREQ=WEEKLY;INTERVAL=2;BYDAY=SA":   "FREQ=WEEKLY;INTERVAL=2;BYDAY=SA",
		"FREQ=MONTHLY;BYDAY=-1FR;COUNT=6":   "FREQ=MONTHLY;BYDAY=-1FR;COUNT=6",
		"FREQ=MONTHLY;BYMONTHDAY=5":         "FREQ=MONTHLY;BYMONTHDAY=5",
		"FREQ=YEARLY;UNTIL=20271231T235959": "FREQ=YEARLY;UNTIL=20271231T235959",
	} {
		if got, err := checkRule(raw); err != nil || got != want {
			t.Errorf("%q: got %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{
		"", "WEEKLY", "FREQ=HOURLY", "BYDAY=MO", "FREQ=DAILY;FREQ=WEEKLY",
		"FREQ=WEEKLY;BYDAY=2TU", "FREQ=WEEKLY;BYDAY=XX", "FREQ=DAILY;INTERVAL=0",
		"FREQ=WEEKLY;BYMONTHDAY=5", "FREQ=DAILY;COUNT=3;UNTIL=20270101",
		"FREQ=MONTHLY;BYSETPOS=-1", "FREQ=DAILY;UNTIL=tomorrow",
	} {
		if _, err := checkRule(raw); err == nil {
			t.Errorf("%q: accepted", raw)
		}
	}
}

func TestWallclockJSON(t *testing.T) {
	var v struct{ At wallclock.Time }
	if err := json.Unmarshal([]byte(`{"At":"2026-10-17T08:00"}`), &v); err != nil || !v.At.Valid {
		t.Fatalf("unmarshal: %v %+v", err, v)
	}
	if b, _ := json.Marshal(v); string(b) != `{"At":"2026-10-17T08:00"}` {
		t.Errorf("marshal: %s", b)
	}
	if err := json.Unmarshal([]byte(`{"At":null}`), &v); err != nil || v.At.Valid {
		t.Errorf("null: %v %+v", err, v)
	}
	if b, _ := json.Marshal(v); string(b) != `{"At":null}` {
		t.Errorf("marshal null: %s", b)
	}
	if err := json.Unmarshal([]byte(`{"At":"2026-10-17T08:00:00Z"}`), &v); err == nil {
		t.Error("a zoned time was accepted")
	}
}
