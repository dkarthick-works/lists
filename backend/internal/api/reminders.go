package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"lists/internal/auth"
	"lists/internal/db"
	"lists/internal/wallclock"
)

// setReminder sets an entry's reminder, or clears it when remind_at is null.
func (s *Server) setReminder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	body, ok := decode[struct {
		RemindAt   wallclock.Time `json:"remind_at"`
		RemindRule *string        `json:"remind_rule"`
	}](w, r)
	if !ok {
		return
	}
	rule := body.RemindRule
	if rule != nil && strings.TrimSpace(*rule) == "" {
		rule = nil
	}
	if rule != nil {
		if !body.RemindAt.Valid {
			writeError(w, http.StatusUnprocessableEntity, "remind_rule needs remind_at")
			return
		}
		norm, err := checkRule(*rule)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		rule = &norm
	}

	item, err := s.q.SetReminder(r.Context(), db.SetReminderParams{
		ID: id, UserID: auth.UserID(r.Context()), RemindAt: body.RemindAt, RemindRule: rule,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// listReminders returns every reminder the user has, for phones to schedule.
func (s *Server) listReminders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListReminders(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

var (
	ruleDay      = regexp.MustCompile(`^(-?[1-5])?(MO|TU|WE|TH|FR|SA|SU)$`)
	ruleUntil    = regexp.MustCompile(`^\d{8}(T\d{6})?$`)
	ruleFreqs    = map[string]bool{"DAILY": true, "WEEKLY": true, "MONTHLY": true, "YEARLY": true}
	errRuleShape = errors.New("remind_rule must be an RRULE such as FREQ=WEEKLY;BYDAY=MO,TH")
)

// checkRule accepts the part of RFC 5545 the picker writes: FREQ (daily to
// yearly), INTERVAL, BYDAY (with an ordinal only when monthly), BYMONTHDAY,
// and COUNT or UNTIL. It returns the rule upper-cased without an "RRULE:"
// prefix, so phones get one spelling.
func checkRule(raw string) (string, error) {
	rule := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(raw)), "RRULE:")
	parts := map[string]string{}
	for _, p := range strings.Split(rule, ";") {
		k, v, ok := strings.Cut(p, "=")
		if !ok || v == "" || parts[k] != "" {
			return "", errRuleShape
		}
		parts[k] = v
	}
	freq := parts["FREQ"]
	if !ruleFreqs[freq] {
		return "", errors.New("remind_rule needs FREQ of DAILY, WEEKLY, MONTHLY or YEARLY")
	}
	for k, v := range parts {
		switch k {
		case "FREQ":
		case "INTERVAL", "COUNT":
			if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 999 {
				return "", fmt.Errorf("%s must be 1 to 999", k)
			}
		case "BYDAY":
			for _, d := range strings.Split(v, ",") {
				m := ruleDay.FindStringSubmatch(d)
				if m == nil || (m[1] != "" && freq != "MONTHLY") {
					return "", fmt.Errorf("BYDAY has an invalid day %q", d)
				}
			}
		case "BYMONTHDAY":
			if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 31 || freq != "MONTHLY" {
				return "", errors.New("BYMONTHDAY must be 1 to 31, with FREQ=MONTHLY")
			}
		case "UNTIL":
			if !ruleUntil.MatchString(v) {
				return "", errors.New("UNTIL must look like 20261231 or 20261231T235959")
			}
		default:
			return "", fmt.Errorf("remind_rule does not support %s", k)
		}
	}
	if parts["COUNT"] != "" && parts["UNTIL"] != "" {
		return "", errors.New("remind_rule cannot have both COUNT and UNTIL")
	}
	return rule, nil
}
