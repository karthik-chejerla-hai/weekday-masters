package main

import (
	"encoding/json"
	"github.com/weekday-masters/backend/internal/services"
	"io"
	"os"
	"strings"
	"testing"
)

func TestTerminalDeliveryFailuresEmitOperationalError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		summary  *services.DeliveryRunSummary
		severity string
	}{
		{"accepted", &services.DeliveryRunSummary{Completed: 1}, "INFO"},
		{"rejected", &services.DeliveryRunSummary{Failed: 1}, "ERROR"},
		{"uncertain", &services.DeliveryRunSummary{Uncertain: 1}, "ERROR"},
		{"partial", &services.DeliveryRunSummary{Partial: 1}, "ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stdout
			os.Stdout = w
			defer func() { os.Stdout = original }()
			emitWorkerHealth("notification_drain", tc.summary, nil)
			w.Close()
			raw, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			if record["severity"] != tc.severity || record["failed"] != (tc.severity == "ERROR") {
				t.Fatal(string(raw))
			}
			if strings.Contains(string(raw), "token") {
				t.Fatal("private data logged")
			}
		})
	}
}
