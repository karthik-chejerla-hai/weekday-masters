package middleware

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/testsupport"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	testsupport.Setup("middleware_test")
	os.Exit(m.Run())
}
