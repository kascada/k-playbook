package webui

import (
	"net/http/httptest"
	"testing"
)

// Temporärer manueller Check — wird nach der Prüfung wieder gelöscht.
func TestManualReviewsPage(t *testing.T) {
	handler := routes(&serverState{shutdown: func() {}})
	req := httptest.NewRequest("GET", "/reviews", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	t.Logf("GET /reviews -> %d\n%s", rec.Code, rec.Body.String())
}
