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
	body := rr.Body.String()
	if !strings.Contains(body, "Waitlist dashboard") {
		t.Fatalf("expected dashboard content")
	}
	if !strings.Contains(body, ">Settings<") {
		t.Fatalf("expected settings navigation in title bar")
	}
	if strings.Contains(body, "Profile status") {
		t.Fatalf("did not expect profile status card on dashboard")
	}
}

func TestHomeRouteRejectsPost(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestItemsNewRouteGet(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/items/new", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Quick capture") {
		t.Fatalf("expected add-item form on /items/new")
	}
}

func TestCreateItemWithOnlyTitle(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Headphones")

	req := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(form.Encode()))
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
	if !strings.Contains(body, "Headphones") {
		t.Fatalf("expected item title in response body")
	}
	if !strings.Contains(body, "Waiting") {
		t.Fatalf("expected item status Waiting in response body")
	}
}

func TestCreateItemValidation(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "")

	req := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Please enter a title.") {
		t.Fatalf("expected validation message in response body")
	}
}

func TestCreateItemWithPresetWaitDuration(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Espresso machine")
	form.Set("wait_preset", "7d")

	req := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(form.Encode()))
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
	if got := item.PurchaseAllowedAt.Sub(item.CreatedAt); got != 7*24*time.Hour {
		t.Fatalf("expected 168h wait duration, got %s", got)
	}
}

func TestCreateItemValidationKeepsCustomHoursVisible(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("title", "Desk")
	form.Set("wait_preset", "custom")
	form.Set("wait_custom_hours", "0")

	req := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "id=\"custom-hours-group\" hidden") {
		t.Fatalf("expected custom hours group to stay visible")
	}
}

func TestStatusAutomaticallyBecomesReadyToBuy(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                1,
		Title:             "Coffee grinder",
		Status:            "Waiting",
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
	if !strings.Contains(rr.Body.String(), "Ready to buy") {
		t.Fatalf("expected promoted status in rendered page")
	}
}

func TestStatusCanBeSetToBoughtFromReadyToBuy(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                42,
		Title:             "Monitor",
		Status:            "Ready to buy",
		CreatedAt:         time.Now().Add(-48 * time.Hour),
		PurchaseAllowedAt: time.Now().Add(-24 * time.Hour),
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "42")
	form.Set("status", "Bought")

	req := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Bought" {
		t.Fatalf("expected status Bought, got %q", app.items[0].Status)
	}
}

func TestStatusUpdateFromWaitingReturnsConflict(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{ID: 5, Title: "Chair", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(24 * time.Hour)})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "5")
	form.Set("status", "Skipped")

	req := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestProfileSettingsGet(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Profile settings") {
		t.Fatalf("expected profile settings page")
	}
}

func TestProfileCanBeSavedAndPersisted(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "42.5")

	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/settings/profile?saved=1" {
		t.Fatalf("unexpected redirect location %q", got)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	getRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRR, getReq)
	if body := getRR.Body.String(); !strings.Contains(body, "value=\"42.5\"") {
		t.Fatalf("expected saved wage visible on profile settings")
	}
}

func TestProfileValidation(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "0")

	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "valid hourly wage") {
		t.Fatalf("expected hourly wage validation in response body")
	}
}

func TestLegacyProfileRouteRedirectsOnGet(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/settings/profile" {
		t.Fatalf("expected redirect to settings page, got %q", got)
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
		{name: "invalid custom", preset: "custom", customHours: "0", wantErrContains: "valid number"},
		{name: "invalid preset", preset: "abc", wantErrContains: "valid wait time"},
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

func TestParseHourlyWage(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		want            float64
		wantErrContains string
	}{
		{name: "valid", raw: "20", want: 20},
		{name: "valid decimal", raw: "17.5", want: 17.5},
		{name: "empty", raw: "", wantErrContains: "valid hourly wage"},
		{name: "zero", raw: "0", wantErrContains: "valid hourly wage"},
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
