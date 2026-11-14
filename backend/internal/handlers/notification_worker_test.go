package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
)

func TestWorkerRequiresExactSchedulerIdentity(t *testing.T) {
	const audience = "https://rally.internal/notifications/test"
	const email = "scheduler@example.iam.gserviceaccount.com"
	for _, tc := range []struct {
		name, bearer, claimEmail, issuer, aud string
		verified                              bool
		expires                               int64
		valid                                 bool
	}{
		{"missing", "", email, "accounts.google.com", audience, true, time.Now().Add(time.Hour).Unix(), false},
		{"member", "Bearer token", "player@example.test", "accounts.google.com", audience, true, time.Now().Add(time.Hour).Unix(), false},
		{"unverified", "Bearer token", email, "accounts.google.com", audience, false, time.Now().Add(time.Hour).Unix(), false},
		{"wrongaudience", "Bearer token", email, "accounts.google.com", "wrong", true, time.Now().Add(time.Hour).Unix(), false},
		{"wrongissuer", "Bearer token", email, "https://untrusted.test", audience, true, time.Now().Add(time.Hour).Unix(), false},
		{"expired", "Bearer token", email, "accounts.google.com", audience, true, time.Now().Add(-time.Hour).Unix(), false},
		{"scheduler", "Bearer token", email, "accounts.google.com", audience, true, time.Now().Add(time.Hour).Unix(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := NewNotificationWorkerRouter(audience, email, func(ctx context.Context, token, aud string) (*idtoken.Payload, error) {
				if token != "token" || aud != audience {
					t.Fatal("unexpected validator arguments")
				}
				return &idtoken.Payload{Audience: tc.aud, Issuer: tc.issuer, Expires: tc.expires, Claims: map[string]interface{}{"email": tc.claimEmail, "email_verified": tc.verified}}, nil
			}, func(ctx context.Context) (any, error) {
				called = true
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded run")
				}
				return map[string]int{"claimed": 1}, nil
			})
			req := httptest.NewRequest(http.MethodPost, "/internal/notifications/run", nil)
			req.Header.Set("Authorization", tc.bearer)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			want := http.StatusUnauthorized
			if tc.valid {
				want = http.StatusOK
			}
			if recorder.Code != want || called != tc.valid {
				t.Fatalf("status=%d called=%v", recorder.Code, called)
			}
		})
	}
}
func TestWorkerDoesNotExposeRunErrors(t *testing.T) {
	router := NewNotificationWorkerRouter("aud", "scheduler", func(context.Context, string, string) (*idtoken.Payload, error) {
		return &idtoken.Payload{Audience: "aud", Issuer: "accounts.google.com", Expires: time.Now().Add(time.Hour).Unix(), Claims: map[string]interface{}{"email": "scheduler", "email_verified": true}}, nil
	}, func(context.Context) (any, error) { return nil, errors.New("private DB/token content") })
	req := httptest.NewRequest(http.MethodPost, "/internal/notifications/run", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable || w.Body.String() != "{\"error\":\"Notification worker run failed.\"}" {
		t.Fatal(w.Code, w.Body.String())
	}
}
