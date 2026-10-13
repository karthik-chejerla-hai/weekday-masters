package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	otellog "go.opentelemetry.io/otel/log"
)

const maxBrowserLogBytes = 8 << 10

type browserLogEvent struct {
	Source    string `json:"source"`
	ErrorType string `json:"error_type"`
}

// BrowserLogsHandler accepts a small allowlisted event from authenticated
// browser clients. It deliberately excludes messages, stack traces, URLs,
// request data, and user identifiers.
func (t *Telemetry) BrowserLogsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
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
