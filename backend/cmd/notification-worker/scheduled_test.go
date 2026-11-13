package main

import (
	"context"
	"errors"
	"github.com/weekday-masters/backend/internal/services"
	"reflect"
	"testing"
)

func TestScheduledWorkerProducesBeforeDrainingAndReportsScanFailure(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		var calls []string
		_, err := runScheduledWork(context.Background(), stopped, func(context.Context) error { calls = append(calls, "scan"); return nil }, func(context.Context, int) (*services.DeliveryRunSummary, error) {
			calls = append(calls, "drain")
			return &services.DeliveryRunSummary{}, nil
		})
		want := []string{"scan", "drain"}
		if stopped {
			want = []string{"drain"}
		}
		if err != nil || !reflect.DeepEqual(calls, want) {
			t.Fatalf("stopped=%v calls=%v err=%v", stopped, calls, err)
		}
	}
	failure := errors.New("database unavailable")
	_, err := runScheduledWork(context.Background(), false, func(context.Context) error { return failure }, func(context.Context, int) (*services.DeliveryRunSummary, error) {
		t.Fatal("drained after failed scan")
		return nil, nil
	})
	if !errors.Is(err, failure) {
		t.Fatalf("lost scan failure: %v", err)
	}
}
