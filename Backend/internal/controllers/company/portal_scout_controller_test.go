package company

import (
	"errors"
	"net/http"
	"testing"

	"Backend/internal/services/companyportal"

	"github.com/labstack/echo/v4"
)

// SOCAIAGENT-694: 企業ごとの送信上限の超過は 429 で返す（500 にしない）。
func TestMapScoutError_QuotaExceeded(t *testing.T) {
	err := mapScoutError(companyportal.ErrScoutQuotaExceeded)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %v", err)
	}
}
