package caldav

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerSupportsOptions(t *testing.T) {
	handler := NewHandler()
	req := httptest.NewRequest(http.MethodOptions, "/caldav", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		t.Fatalf("expected 2xx response for OPTIONS, got %d", rec.Code)
	}
}

func TestNewHandlerSupportsPutAndGet(t *testing.T) {
	handler := NewHandler()

	icsPayload := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"
	putReq := httptest.NewRequest(http.MethodPut, "/caldav/default.ics", strings.NewReader(icsPayload))
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusCreated && putRec.Code != http.StatusNoContent {
		t.Fatalf("expected PUT status 201/204, got %d", putRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/caldav/default.ics", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d", getRec.Code)
	}

	body, err := io.ReadAll(getRec.Body)
	if err != nil {
		t.Fatalf("read GET body: %v", err)
	}

	if string(body) != icsPayload {
		t.Fatalf("unexpected GET body, got %q", string(body))
	}
}
