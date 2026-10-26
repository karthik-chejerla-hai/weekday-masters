package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMemberMoneyStrictEnvelope(t *testing.T) {
	for _, raw := range []string{
		`{"request_id":"bad","proposed":{}}`,
		`{"actor_id":"not-trusted","proposed":{}}`,
		`{"proposed":{},"proposed":{"amount_cents":100}}`,
		`{"proposed":{"amount_cents":1.5}}`,
		`{"proposed":{"amount_cents":4503599627370496}}`,
		`{"proposed":{"x":1,"x":2}}`,
		`{"proposed":{}} {"proposed":{}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var request memberMoneyRequest
			if err := decodeMemberMoneyJSON(json.RawMessage(raw), &request); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	var request memberMoneyRequest
	if err := decodeMemberMoneyJSON(json.RawMessage(`{"request_id":"00000000-0000-0000-0000-000000000001","proposed":{"amount_cents":100}}`), &request); err != nil {
		t.Fatal(err)
	}
	var proposal struct {
		AmountCents int64 `json:"amount_cents"`
	}
	if err := decodeMemberMoneyJSON(json.RawMessage(`{"amount_cents":100,"surprise":true}`), &proposal); err == nil {
		t.Fatal("unknown nested proposal field accepted")
	}
}

func TestMemberMoneyRejectsOversizedBody(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"proposed":{"note":"`+strings.Repeat("x", 128*1024)+`"}}`))
	var request memberMoneyRequest
	if err := decodeMemberMoneyRequest(c, &request); err == nil {
		t.Fatal("oversized request accepted")
	}
}
