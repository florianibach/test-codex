package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHomeRoute(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHomeRouteHead(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAssetsRoute(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("expected text/css content type, got %s", contentType)
	}
	if body := rr.Body.String(); !strings.Contains(body, ".btn-primary") {
		t.Fatalf("expected css body content")
	}
}

func TestCreateItemWithOnlyTitle(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Kopfhörer")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("expected redirect to /, got %s", got)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRR, getReq)

	body := getRR.Body.String()
	if !strings.Contains(body, "Kopfhörer") {
		t.Fatalf("expected item title in response body")
	}
	if !strings.Contains(body, "Wartet") {
		t.Fatalf("expected item status Wartet in response body")
	}
}

func TestCreateItemValidation(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Bitte gib einen Titel ein.") {
		t.Fatalf("expected validation message in response body")
	}
}

func TestCreateItemWithPresetWaitDuration(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Espressomaschine")
	form.Set("wait_preset", "7d")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if len(app.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(app.items))
	}
	item := app.items[0]
	if item.WaitPreset != "7d" {
		t.Fatalf("expected wait preset 7d, got %q", item.WaitPreset)
	}
	delta := item.PurchaseAllowedAt.Sub(item.CreatedAt)
	if delta != 7*24*time.Hour {
		t.Fatalf("expected 168h wait duration, got %s", delta)
	}
}

func TestCreateItemWithCustomWaitDuration(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Gaming Maus")
	form.Set("wait_preset", "custom")
	form.Set("wait_custom_hours", "12")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	item := app.items[0]
	if item.WaitPreset != "custom" {
		t.Fatalf("expected wait preset custom, got %q", item.WaitPreset)
	}
	if item.WaitCustomHours != "12" {
		t.Fatalf("expected custom hours 12, got %q", item.WaitCustomHours)
	}
	if got := item.PurchaseAllowedAt.Sub(item.CreatedAt); got != 12*time.Hour {
		t.Fatalf("expected 12h wait duration, got %s", got)
	}
}

func TestCreateItemValidationForCustomWaitDuration(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Schreibtisch")
	form.Set("wait_preset", "custom")
	form.Set("wait_custom_hours", "0")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "gültige Anzahl Stunden") {
		t.Fatalf("expected custom validation message in response body")
	}
}

func TestHomeRouteHidesCustomHoursByDefault(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "id=\"custom-hours-group\" hidden") {
		t.Fatalf("expected custom hours group to be hidden by default")
	}
	if !strings.Contains(body, "id=\"wait_custom_hours\"") || !strings.Contains(body, "disabled") {
		t.Fatalf("expected custom hours input to be disabled by default")
	}
}

func TestCreateItemValidationKeepsCustomHoursVisible(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Schreibtisch")
	form.Set("wait_preset", "custom")
	form.Set("wait_custom_hours", "0")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "id=\"custom-hours-group\" hidden") {
		t.Fatalf("expected custom hours group to stay visible on custom validation error")
	}
	if strings.Contains(body, "id=\"wait_custom_hours\" name=\"wait_custom_hours\" type=\"number\" class=\"form-control\" placeholder=\"z. B. 12\" value=\"0\" disabled") {
		t.Fatalf("expected custom hours input to remain enabled on custom validation error")
	}
}

func TestParseWaitDuration(t *testing.T) {
	tests := []struct {
		name            string
		preset          string
		customHours     string
		wantDuration    time.Duration
		wantErrContains string
	}{
		{name: "default", preset: "", wantDuration: 24 * time.Hour},
		{name: "24h", preset: "24h", wantDuration: 24 * time.Hour},
		{name: "7d", preset: "7d", wantDuration: 7 * 24 * time.Hour},
		{name: "30d", preset: "30d", wantDuration: 30 * 24 * time.Hour},
		{name: "custom", preset: "custom", customHours: "5", wantDuration: 5 * time.Hour},
		{name: "invalid custom", preset: "custom", customHours: "0", wantErrContains: "gültige Anzahl Stunden"},
		{name: "invalid preset", preset: "abc", wantErrContains: "gültige Wartezeit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWaitDuration(tt.preset, tt.customHours)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantDuration {
				t.Fatalf("expected %s, got %s", tt.wantDuration, got)
			}
		})
	}
}

func TestHealthRoute(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf("unexpected body %s", body)
	}
}

func TestUnknownRoute(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
