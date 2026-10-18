// Package telemetry exports application logs and metrics over OTLP/HTTP.
package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Telemetry struct {
	logs        *sdklog.LoggerProvider
	metrics     *sdkmetric.MeterProvider
	logger      otellog.Logger
	browserLogs *browserLogLimiter
	requests    metric.Int64Counter
	duration    metric.Float64Histogram
	console     io.Writer
	flags       int
}

// Start is opt-in. Standard exporter environment variables configure endpoint,
// headers and timeouts; credentials never need to enter application config.
func Start(ctx context.Context) (*Telemetry, error) {
	t := &Telemetry{console: log.Writer(), flags: log.Flags(), browserLogs: newBrowserLogLimiter()}
	if os.Getenv("OTEL_ENABLED") != "true" {
		return t, nil
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, errors.New("OTEL_ENABLED requires OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	// Export failures must bypass the log bridge to avoid recursive export.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Fprintln(os.Stderr, "OpenTelemetry export failed:", err)
	}))
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.instance.id", uuid.NewString())))
	if err != nil {
		return nil, err
	}
	if _, ok := os.LookupEnv("OTEL_SERVICE_NAME"); !ok {
		res, err = resource.Merge(resource.NewSchemaless(attribute.String("service.name", "rally-api")), res)
		if err != nil {
			return nil, err
		}
	}
	le, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	t.logs = sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(le)))
	me, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = t.logs.Shutdown(ctx)
		return nil, err
	}
	t.metrics = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)))
	meter := t.metrics.Meter("rally-api")
	t.requests, err = meter.Int64Counter("http.server.requests", metric.WithUnit("{request}"))
	if err == nil {
		t.duration, err = meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10))
	}
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	heap, err := meter.Int64ObservableGauge("go.memory.heap", metric.WithUnit("By"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	gr, err := meter.Int64ObservableGauge("go.goroutines", metric.WithUnit("{goroutine}"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		o.ObserveInt64(heap, int64(m.HeapAlloc))
		o.ObserveInt64(gr, int64(runtime.NumGoroutine()))
		return nil
	}, heap, gr)
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, err
	}
	t.logger = t.logs.Logger("rally-api")
	log.SetFlags(0)
	log.SetOutput(t)
	return t, nil
}

// Write retains console output and bridges existing standard-library logs.
func (t *Telemetry) Write(p []byte) (int, error) {
	n, err := t.console.Write(p)
	if t.logger != nil {
		var r otellog.Record
		r.SetTimestamp(time.Now())
		r.SetObservedTimestamp(time.Now())
		r.SetSeverity(otellog.SeverityInfo)
		r.SetBody(otellog.StringValue(strings.TrimSpace(string(p))))
		t.logger.Emit(context.Background(), r)
	}
	return n, err
}

// Middleware uses route templates, never raw paths, query strings, headers or
// request bodies. This avoids member identifiers and unbounded metric labels.
func (t *Telemetry) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		switch method {
		case "GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH":
		default:
			method = "OTHER"
		}
		status := c.Writer.Status()
		elapsed := time.Since(start).Seconds()
		if t.metrics != nil {
			attrs := metric.WithAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			t.requests.Add(c.Request.Context(), 1, attrs)
			t.duration.Record(c.Request.Context(), elapsed, attrs)
		}
		log.Printf("HTTP %s %s status=%d duration_seconds=%.6f", method, route, status, elapsed)
	}
}

func (t *Telemetry) ObserveDB(db *sql.DB) error {
	if t.metrics == nil {
		return nil
	}
	m := t.metrics.Meter("rally-api")
	connections, err := m.Int64ObservableGauge("db.client.connections", metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	waits, err := m.Int64ObservableCounter("db.client.connection.waits", metric.WithUnit("{wait}"))
	if err != nil {
		return err
	}
	_, err = m.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		s := db.Stats()
		o.ObserveInt64(connections, int64(s.InUse), metric.WithAttributes(attribute.String("state", "used")))
		o.ObserveInt64(connections, int64(s.Idle), metric.WithAttributes(attribute.String("state", "idle")))
		o.ObserveInt64(waits, s.WaitCount)
		return nil
	}, connections, waits)
	return err
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.logs == nil {
		return nil
	}
	log.SetOutput(t.console)
	log.SetFlags(t.flags)
	// Shut down metrics first while the log provider is still available.
	var metricErr error
	if t.metrics != nil {
		metricErr = t.metrics.Shutdown(ctx)
	}
	return errors.Join(metricErr, t.logs.Shutdown(ctx))
}
