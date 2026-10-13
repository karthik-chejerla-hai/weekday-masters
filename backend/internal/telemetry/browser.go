package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/models"
	otellog "go.opentelemetry.io/otel/log"
)

const maxBrowserLogBytes = 8 << 10

const (
	browserLogBurst        = 40.0
	browserLogRefillPerSec = 1.0
	browserLogIdleTimeout  = 10 * time.Minute
)

type browserLogBucket struct {
	tokens  float64
	updated time.Time
}

type browserLogLimiter struct {
	mu        sync.Mutex
	users     map[string]browserLogBucket
	lastSweep time.Time
}

func newBrowserLogLimiter() *browserLogLimiter {
	return &browserLogLimiter{users: make(map[string]browserLogBucket)}
}

// allow limits event volume per authenticated member. It allows a short burst
// of 40 events, then refills at one event per second.
func (l *browserLogLimiter) allow(userID string, count int, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) >= browserLogIdleTimeout {
		for id, bucket := range l.users {
			if now.Sub(bucket.updated) >= browserLogIdleTimeout {
				delete(l.users, id)
			}
		}
		l.lastSweep = now
	}

	bucket, ok := l.users[userID]
	if !ok {
		bucket = browserLogBucket{tokens: browserLogBurst, updated: now}
	}
	bucket.tokens += now.Sub(bucket.updated).Seconds() * browserLogRefillPerSec
	if bucket.tokens > browserLogBurst {
		bucket.tokens = browserLogBurst
	}
	bucket.updated = now
	if float64(count) > bucket.tokens {
		wait := time.Duration((float64(count) - bucket.tokens) / browserLogRefillPerSec * float64(time.Second))
		l.users[userID] = bucket
		return false, wait
	}
	bucket.tokens -= float64(count)
	l.users[userID] = bucket
	return true, 0
}

type browserLogEvent struct {
	Source    string `json:"source"`
	ErrorType string `json:"error_type"`
}

// BrowserLogsHandler accepts a small allowlisted event from authenticated
// browser clients. It deliberately excludes messages, stack traces, URLs,
// request data, and user identifiers.
func (t *Telemetry) BrowserLogsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		current, exists := c.Get("user")
		user, ok := current.(*models.User)
		if !exists || !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBrowserLogBytes)
		var events []browserLogEvent
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&events); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid browser log batch"})
			return
		}
		if len(events) == 0 || len(events) > 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid browser log batch"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid browser log batch"})
			return
		}
		for _, event := range events {
			if !validBrowserLogSource(event.Source) || !validBrowserErrorType(event.ErrorType) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid browser log event"})
				return
			}
		}
		if t.browserLogs == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		allowed, retryAfter := t.browserLogs.allow(user.ID.String(), len(events), time.Now())
		if !allowed {
			retryAfterSeconds := int((retryAfter + time.Second - 1) / time.Second)
			c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "browser log rate limit exceeded"})
			return
		}
		for _, event := range events {
			t.recordBrowserLog(event)
		}
		c.Status(http.StatusAccepted)
	}
}

func validBrowserLogSource(source string) bool {
	switch source {
	case "console.error", "window.error", "unhandledrejection":
		return true
	default:
		return false
	}
}

func validBrowserErrorType(name string) bool {
	// Browser clients only send JavaScript's built-in error class names. This
	// keeps arbitrary message content out of log attributes.
	switch name {
	case "Error", "EvalError", "RangeError", "ReferenceError", "SyntaxError", "TypeError", "URIError", "AggregateError", "ConsoleError", "NonErrorRejection":
		return true
	default:
		return false
	}
}

func (t *Telemetry) recordBrowserLog(event browserLogEvent) {
	if t.logger == nil {
		return
	}
	var record otellog.Record
	record.SetTimestamp(time.Now())
	record.SetObservedTimestamp(time.Now())
	record.SetSeverity(otellog.SeverityError)
	record.SetBody(otellog.StringValue("Browser client error"))
	record.AddAttributes(
		otellog.KeyValue{Key: "browser.error.source", Value: otellog.StringValue(event.Source)},
		otellog.KeyValue{Key: "browser.error.type", Value: otellog.StringValue(event.ErrorType)},
		otellog.KeyValue{Key: "event.domain", Value: otellog.StringValue("browser")},
	)
	t.logger.Emit(context.Background(), record)
}
