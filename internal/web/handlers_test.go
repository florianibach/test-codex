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

func TestStatusAutomaticallyBecomesPurchaseAllowed(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                1,
		Title:             "Kaffeemaschine",
		Status:            "Wartet",
		CreatedAt:         time.Now().Add(-2 * time.Hour),
		PurchaseAllowedAt: time.Now().Add(-time.Minute),
	})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Kauf erlaubt") {
		t.Fatalf("expected item status Kauf erlaubt to be rendered")
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Kauf erlaubt" {
		t.Fatalf("expected status to be promoted, got %q", app.items[0].Status)
	}
}

func TestStatusCanBeSetToBoughtFromPurchaseAllowed(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                42,
		Title:             "Monitor",
		Status:            "Kauf erlaubt",
		CreatedAt:         time.Now().Add(-48 * time.Hour),
		PurchaseAllowedAt: time.Now().Add(-24 * time.Hour),
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "42")
	form.Set("status", "Gekauft")

	req := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Gekauft" {
		t.Fatalf("expected status Gekauft, got %q", app.items[0].Status)
	}
}

func TestStatusCanBeSetToNotBoughtFromPurchaseAllowed(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                9,
		Title:             "Sneaker",
		Status:            "Kauf erlaubt",
		CreatedAt:         time.Now().Add(-48 * time.Hour),
		PurchaseAllowedAt: time.Now().Add(-24 * time.Hour),
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "9")
	form.Set("status", "Nicht gekauft")

	req := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Nicht gekauft" {
		t.Fatalf("expected status Nicht gekauft, got %q", app.items[0].Status)
	}
}

func TestStatusUpdateFromWaitingReturnsConflict(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                5,
		Title:             "Stuhl",
		Status:            "Wartet",
		CreatedAt:         time.Now(),
		PurchaseAllowedAt: time.Now().Add(24 * time.Hour),
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "5")
	form.Set("status", "Nicht gekauft")

	req := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestTerminalStatusDoesNotRevertDuringPromotion(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Laptop", Status: "Gekauft", PurchaseAllowedAt: time.Now().Add(-time.Hour)},
		Item{ID: 2, Title: "Headset", Status: "Nicht gekauft", PurchaseAllowedAt: time.Now().Add(-time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Gekauft" {
		t.Fatalf("expected first item to remain Gekauft, got %q", app.items[0].Status)
	}
	if app.items[1].Status != "Nicht gekauft" {
		t.Fatalf("expected second item to remain Nicht gekauft, got %q", app.items[1].Status)
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
		{name: "custom decimal", preset: "custom", customHours: "0.5", wantDuration: 30 * time.Minute},
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

func TestProfileCanBeSavedAndPersisted(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "42.5")

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Profil gespeichert.") {
		t.Fatalf("expected success feedback in response body")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRR, getReq)

	if body := getRR.Body.String(); !strings.Contains(body, "value=\"42.5\"") {
		t.Fatalf("expected persisted hourly wage in profile form")
	}
}

func TestProfileValidation(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "0")

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "gültigen Stundenlohn") {
		t.Fatalf("expected hourly wage validation in response body")
	}
}

func TestProfileRouteMethodNotAllowed(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestParseHourlyWage(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		want            float64
		wantErrContains string
	}{
		{name: "valid", raw: "20", want: 20},
		{name: "valid decimal", raw: "17.5", want: 17.5},
		{name: "trimmed", raw: " 33 ", want: 33},
		{name: "empty", raw: "", wantErrContains: "gültigen Stundenlohn"},
		{name: "zero", raw: "0", wantErrContains: "gültigen Stundenlohn"},
		{name: "negative", raw: "-2", wantErrContains: "gültigen Stundenlohn"},
		{name: "not numeric", raw: "abc", wantErrContains: "gültigen Stundenlohn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHourlyWage(tt.raw)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
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
