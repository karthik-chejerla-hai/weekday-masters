package telemetry

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	logspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

func TestDisabledAndMissingEndpoint(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "false")
	original := log.Writer()
	obs, err := Start(context.Background())
	if err != nil || obs.logs != nil || log.Writer() != original {
		t.Fatal("disabled telemetry changed logging")
	}
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if _, err = Start(context.Background()); err == nil {
		t.Fatal("missing endpoint accepted")
	}
}

// Exercise the actual exporters, including base-path suffixes, authorization,
// protobuf payloads and shutdown flushing, without Grafana credentials or a DB.
func TestOTLPExport(t *testing.T) {
	var mu sync.Mutex
	var logs logspb.ExportLogsServiceRequest
	var metrics metricspb.ExportMetricsServiceRequest
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic test" {
			t.Error("missing authorization")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/otlp/v1/logs":
			var batch logspb.ExportLogsServiceRequest
			if err := proto.Unmarshal(body, &batch); err != nil {
				t.Error(err)
			}
			logs.ResourceLogs = append(logs.ResourceLogs, batch.ResourceLogs...)
		case "/otlp/v1/metrics":
			if err := proto.Unmarshal(body, &metrics); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected OTLP path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer receiver.Close()
	for _, signal := range []string{"LOGS", "METRICS"} {
		for _, field := range []string{"ENDPOINT", "HEADERS", "INSECURE"} {
			t.Setenv("OTEL_EXPORTER_OTLP_"+signal+"_"+field, "")
		}
	}
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_SERVICE_NAME", "rally-test")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name=test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL+"/otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Basic%20test")
	var console bytes.Buffer
	old := log.Writer()
	log.SetOutput(&console)
	defer log.SetOutput(old)
	obs, err := Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })
	log.Print("application started")
	router := gin.New()
	router.Use(obs.Middleware())
	router.GET("/members/:id", func(c *gin.Context) { c.Status(204) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/members/private-member?token=private-secret", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/unknown-private-member?token=private-secret", nil))
	if err := obs.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	serialized := logs.String() + metrics.String() + console.String()
	if strings.Contains(serialized, "private-member") || strings.Contains(serialized, "private-secret") {
		t.Fatal("raw request data leaked")
	}
	for _, expected := range []string{"rally-test", "application started", "/members/:id", "unmatched"} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("missing log %q", expected)
		}
	}
	for _, expected := range []string{"http.server.requests", "http.server.request.duration", "go.memory.heap", "go.goroutines"} {
		if !strings.Contains(metrics.String(), expected) {
			t.Errorf("missing metric %q", expected)
		}
	}
	if !strings.Contains(console.String(), "application started") {
		t.Error("console logging lost")
	}
}
