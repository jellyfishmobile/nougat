package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketUsageTotals  = "UsageTotals"
	bucketUsageHistory = "UsageHistory"
	maxHistoryPerKey   = 500
)

// UsageTotals persisted per API key id.
type UsageTotals struct {
	TotalRequests            int64     `json:"total_requests"`
	TotalPromptTokens        int64     `json:"total_prompt_tokens"`
	TotalCompletionTokens    int64     `json:"total_completion_tokens"`
	LastRequestAt            time.Time `json:"last_request_at"`
	FirstRecordedAt          time.Time `json:"first_recorded_at"`
}

// UsageEvent is one line in the per-key history list.
type UsageEvent struct {
	At               time.Time `json:"at"`
	Path             string    `json:"path"`
	Method           string    `json:"method"`
	HTTPStatus       int       `json:"http_status"`
	Model            string    `json:"model,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	DurationMs       int64     `json:"duration_ms"`
	ErrorSnippet     string    `json:"error_snippet,omitempty"`
}

func recordAPIUsage(db *bolt.DB, keyID string, ev UsageEvent) error {
	if keyID == "" {
		return nil
	}
	return db.Update(func(tx *bolt.Tx) error {
		tb := tx.Bucket([]byte(bucketUsageTotals))
		var totals UsageTotals
		if raw := tb.Get([]byte(keyID)); len(raw) > 0 {
			_ = json.Unmarshal(raw, &totals)
		}
		if totals.FirstRecordedAt.IsZero() {
			totals.FirstRecordedAt = ev.At
		}
		totals.TotalRequests++
		totals.TotalPromptTokens += int64(ev.PromptTokens)
		totals.TotalCompletionTokens += int64(ev.CompletionTokens)
		totals.LastRequestAt = ev.At
		raw, err := json.Marshal(totals)
		if err != nil {
			return err
		}
		if err := tb.Put([]byte(keyID), raw); err != nil {
			return err
		}

		hb := tx.Bucket([]byte(bucketUsageHistory))
		var hist []UsageEvent
		if hraw := hb.Get([]byte(keyID)); len(hraw) > 0 {
			_ = json.Unmarshal(hraw, &hist)
		}
		hist = append(hist, ev)
		if len(hist) > maxHistoryPerKey {
			hist = hist[len(hist)-maxHistoryPerKey:]
		}
		hdata, err := json.Marshal(hist)
		if err != nil {
			return err
		}
		return hb.Put([]byte(keyID), hdata)
	})
}

func getUsageSummary(db *bolt.DB, keyID string) (UsageTotals, error) {
	var totals UsageTotals
	err := db.View(func(tx *bolt.Tx) error {
		tb := tx.Bucket([]byte(bucketUsageTotals))
		if tb == nil {
			return nil
		}
		raw := tb.Get([]byte(keyID))
		if len(raw) == 0 {
			return nil
		}
		return json.Unmarshal(raw, &totals)
	})
	return totals, err
}

func getUsageHistory(db *bolt.DB, keyID string, limit int) ([]UsageEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxHistoryPerKey {
		limit = maxHistoryPerKey
	}
	var hist []UsageEvent
	err := db.View(func(tx *bolt.Tx) error {
		hb := tx.Bucket([]byte(bucketUsageHistory))
		if hb == nil {
			return nil
		}
		raw := hb.Get([]byte(keyID))
		if len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, &hist); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(hist) > limit {
		hist = hist[len(hist)-limit:]
	}
	// return newest first for dashboard
	out := make([]UsageEvent, len(hist))
	for i := range hist {
		out[len(hist)-1-i] = hist[i]
	}
	return out, nil
}

// errSnippet truncates err for storage.
func errSnippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// dashboardSummaryJSON response shape
type dashboardSummaryJSON struct {
	KeyID  string      `json:"key_id"`
	Label  string      `json:"label"`
	Totals UsageTotals `json:"totals"`
}

func (s *server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := apiKeyIDFromContext(r.Context())
	if keyID == "" {
		writeOpenAIAPIError(w, http.StatusUnauthorized, "missing API key context", "invalid_request_error")
		return
	}
	label := apiKeyLabelFromContext(r.Context())
	totals, err := getUsageSummary(s.db, keyID)
	if err != nil {
		log.Printf("dashboard summary: %v", err)
		writeOpenAIAPIError(w, http.StatusInternalServerError, "could not load stats", "server_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dashboardSummaryJSON{
		KeyID:  keyID,
		Label:  label,
		Totals: totals,
	})
}

func (s *server) handleDashboardHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := apiKeyIDFromContext(r.Context())
	if keyID == "" {
		writeOpenAIAPIError(w, http.StatusUnauthorized, "missing API key context", "invalid_request_error")
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		var n int
		_, _ = fmt.Sscanf(q, "%d", &n)
		if n > 0 {
			limit = n
		}
	}
	hist, err := getUsageHistory(s.db, keyID, limit)
	if err != nil {
		log.Printf("dashboard history: %v", err)
		writeOpenAIAPIError(w, http.StatusInternalServerError, "could not load history", "server_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"key_id":  keyID,
		"events":  hist,
		"count":   len(hist),
		"limited": true,
	})
}
