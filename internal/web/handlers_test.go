package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func seedProfile(app *App) {
	app.mu.Lock()
	app.hourlyWage = "25"
	app.mu.Unlock()
}

func TestHomeRoute(t *testing.T) {
	app := NewApp()
	seedProfile(app)
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
	if strings.Contains(body, "Track how your pause decisions impact your spending habits.") {
		t.Fatalf("did not expect insights page content on dashboard")
	}
}

func TestHomeRedirectsToProfileWhenMissingProfile(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/settings/profile" {
		t.Fatalf("expected redirect to /settings/profile, got %q", got)
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

func TestInsightsRouteGet(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/insights", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<h1 class=\"h3 mb-1\">Insights</h1>") {
		t.Fatalf("expected insights page content")
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
	if body := rr.Body.String(); !strings.Contains(body, "Add item") {
		t.Fatalf("expected add-item form on /items/new")
	}
}

func TestCreateItemWithOnlyTitle(t *testing.T) {
	app := NewApp()
	seedProfile(app)
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

func TestHomeShowsWorkHoursWhenPriceAndHourlyWageArePresent(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.hourlyWage = "25"
	app.items = append(app.items, Item{ID: 1, Title: "Headphones", Price: "100", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(24 * time.Hour)})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Work hours: 4.0 h") {
		t.Fatalf("expected work hours value in response body")
	}
}

func TestHomeShowsNeutralWorkHoursHintWhenDataMissing(t *testing.T) {
	app := NewApp()
	app.mu.Lock()
	app.hourlyWage = "foo"
	app.mu.Unlock()

	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Headphones", Price: "100", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(24 * time.Hour)})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Work hours: add a valid price and hourly wage.") {
		t.Fatalf("expected neutral work hours hint in response body")
	}
}

func TestHomeDoesNotShowWorkHoursSectionWithoutPrice(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.hourlyWage = "25"
	app.items = append(app.items, Item{ID: 1, Title: "Headphones", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(24 * time.Hour)})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "Work hours:") {
		t.Fatalf("did not expect work hours text without price")
	}
}

func TestHomeFilterPanelIsCollapsedByDefault(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<details class=\"mb-3\"") {
		t.Fatalf("expected filter details wrapper")
	}
	if strings.Contains(body, "<details class=\"mb-3\" open>") {
		t.Fatalf("expected filter details to be collapsed by default")
	}
	if !strings.Contains(body, "data-auto-submit-filter=\"true\"") {
		t.Fatalf("expected auto-submit filter form marker")
	}
	if strings.Contains(body, ">Apply<") {
		t.Fatalf("did not expect manual apply button")
	}
	if !strings.Contains(body, "data-status-all=\"true\"") {
		t.Fatalf("expected all-status shortcut button")
	}
}

func TestHomeFilterPanelOpensWhenFiltersAreActive(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	req := httptest.NewRequest(http.MethodGet, "/?q=test", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<details class=\"mb-3\" open>") {
		t.Fatalf("expected filter details to be open when filters are active")
	}
}


func TestHomeFilterPanelStaysOpenForExplicitAllStatuses(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	req := httptest.NewRequest(http.MethodGet, "/?status=Waiting&status=Ready+to+buy&status=Bought&status=Skipped", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<details class=\"mb-3\" open>") {
		t.Fatalf("expected filter details to stay open for explicit all-status selection")
	}
}

func TestHomeFiltersBySearchStatusAndTag(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Laptop", Note: "Work machine", Tags: "tech", Status: "Ready to buy", CreatedAt: now.Add(-2 * time.Hour), PurchaseAllowedAt: now.Add(-1 * time.Hour)},
		Item{ID: 2, Title: "Shoes", Note: "Running", Tags: "sport", Status: "Waiting", CreatedAt: now.Add(-1 * time.Hour), PurchaseAllowedAt: now.Add(24 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/?q=work&status=Ready+to+buy&tag=tech", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Laptop") {
		t.Fatalf("expected filtered item to be present")
	}
	if strings.Contains(body, "Shoes") {
		t.Fatalf("did not expect non-matching item to be present")
	}
}

func TestHomeSortsByPriceAscending(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "High", Price: "100", PriceValue: 100, HasPriceValue: true, Status: "Waiting", CreatedAt: now.Add(-2 * time.Hour), PurchaseAllowedAt: now.Add(24 * time.Hour)},
		Item{ID: 2, Title: "Low", Price: "10", PriceValue: 10, HasPriceValue: true, Status: "Waiting", CreatedAt: now.Add(-1 * time.Hour), PurchaseAllowedAt: now.Add(24 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/?sort=price_asc", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	lowIdx := strings.Index(body, "Low")
	highIdx := strings.Index(body, "High")
	if lowIdx == -1 || highIdx == -1 {
		t.Fatalf("expected both items to be present")
	}
	if lowIdx > highIdx {
		t.Fatalf("expected low-priced item to appear before high-priced item")
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
	seedProfile(app)

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

func TestParsePurchaseAllowedAtWithTimezoneOffset(t *testing.T) {
	parsed, err := parsePurchaseAllowedAt("2026-02-12T10:30", "-120")
	if err != nil {
		t.Fatalf("expected valid datetime, got %v", err)
	}
	if got := parsed.Format(time.RFC3339); got != "2026-02-12T10:30:00+02:00" {
		t.Fatalf("unexpected parsed datetime %q", got)
	}
}

func TestParsePurchaseAllowedAtRejectsInvalidTimezoneOffset(t *testing.T) {
	if _, err := parsePurchaseAllowedAt("2026-02-12T10:30", "oops"); err == nil {
		t.Fatalf("expected timezone parse error")
	}
}

func TestCreateItemWithSpecificDateWaitPreset(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	buyAfter := time.Now().Add(6 * time.Hour).Format("2006-01-02T15:04")

	form := url.Values{}
	form.Set("title", "Specific date item")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", buyAfter)

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
	if app.items[0].WaitPreset != "date" {
		t.Fatalf("expected wait preset date, got %q", app.items[0].WaitPreset)
	}
}

func TestCreateItemWithSpecificDateUsesLocalTimezone(t *testing.T) {
	originalLocal := time.Local
	loc := time.FixedZone("UTC+1", 1*60*60)
	time.Local = loc
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	app := NewApp()
	seedProfile(app)

	form := url.Values{}
	form.Set("title", "Timezone item")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", "2026-01-15T19:45")

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

	got := app.items[0].PurchaseAllowedAt
	if got.Location() != loc {
		t.Fatalf("expected parsed location %q, got %q", loc.String(), got.Location().String())
	}
	if got.Hour() != 19 || got.Minute() != 45 {
		t.Fatalf("expected local 19:45, got %02d:%02d", got.Hour(), got.Minute())
	}
}

func TestCreateItemWithSpecificDateUsesBrowserTimezoneOffset(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	app := NewApp()
	seedProfile(app)

	form := url.Values{}
	form.Set("title", "Timezone offset item")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", "2026-01-15T19:45")
	form.Set("timezone_offset_minutes", "-60")

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

	gotUTC := app.items[0].PurchaseAllowedAt.UTC()
	if gotUTC.Hour() != 18 || gotUTC.Minute() != 45 {
		t.Fatalf("expected UTC 18:45 for browser offset -60, got %02d:%02d", gotUTC.Hour(), gotUTC.Minute())
	}
}

func TestCreateItemWithSpecificDateRequiresDateInput(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	form := url.Values{}
	form.Set("title", "Specific date item")
	form.Set("wait_preset", "date")

	req := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Please enter a buy-after date and time.") {
		t.Fatalf("expected missing buy-after validation")
	}
}

func TestEditSkippedItemReevaluatesToWaitingWhenWaitChangesToFuture(t *testing.T) {
	app := NewApp()
	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Original", Status: "Skipped", WaitPreset: "24h", PurchaseAllowedAt: now.Add(-time.Hour), CreatedAt: now, NtfyAttempted: true})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "Original")
	form.Set("wait_preset", "custom")
	form.Set("wait_custom_hours", "5")

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if got := app.items[0].Status; got != "Waiting" {
		t.Fatalf("expected status Waiting, got %q", got)
	}
	if app.items[0].NtfyAttempted {
		t.Fatalf("expected ntfy attempt reset for waiting item")
	}
}

func TestEditItemUpdatesFieldsAndStatusToWaitingWhenBuyAfterInFuture(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	now := time.Now()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                1,
		Title:             "Old title",
		Price:             "100",
		Status:            "Ready to buy",
		WaitPreset:        "24h",
		PurchaseAllowedAt: now.Add(-time.Hour),
		CreatedAt:         now.Add(-48 * time.Hour),
		NtfyAttempted:     true,
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "New title")
	form.Set("price", "150.50")
	form.Set("note", "updated")
	form.Set("link", "https://example.com")
	form.Set("tags", "tech")
	form.Set("wait_preset", "7d")

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	item := app.items[0]
	if item.Title != "New title" || item.Note != "updated" || item.Link != "https://example.com" || item.Tags != "tech" {
		t.Fatalf("expected updated fields, got %+v", item)
	}
	if item.Status != "Waiting" {
		t.Fatalf("expected status Waiting, got %q", item.Status)
	}
	if item.NtfyAttempted {
		t.Fatalf("expected ntfy attempted reset after moving to future")
	}
}

func TestEditItemValidationLeavesItemUnchanged(t *testing.T) {
	app := NewApp()
	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Original", Status: "Waiting", WaitPreset: "24h", PurchaseAllowedAt: now.Add(24 * time.Hour), CreatedAt: now})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "")
	form.Set("wait_preset", "24h")

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Please enter a title.") {
		t.Fatalf("expected title validation error")
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Title != "Original" {
		t.Fatalf("expected unchanged item title, got %q", app.items[0].Title)
	}
}

func TestEditItemSetsStatusToReadyToBuyWhenBuyAfterIsInPast(t *testing.T) {
	app := NewApp()
	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Original", Status: "Waiting", WaitPreset: "24h", PurchaseAllowedAt: now.Add(24 * time.Hour), CreatedAt: now})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "Original")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", now.Add(-2*time.Hour).Format("2006-01-02T15:04"))

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if got := app.items[0].Status; got != "Ready to buy" {
		t.Fatalf("expected status Ready to buy, got %q", got)
	}
}

func TestEditItemSpecificDateUsesBrowserTimezoneOffset(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	app := NewApp()
	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Original", Status: "Waiting", WaitPreset: "24h", PurchaseAllowedAt: now.Add(24 * time.Hour), CreatedAt: now})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "Original")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", "2026-01-15T19:45")
	form.Set("timezone_offset_minutes", "-60")

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	gotUTC := app.items[0].PurchaseAllowedAt.UTC()
	if gotUTC.Hour() != 18 || gotUTC.Minute() != 45 {
		t.Fatalf("expected UTC 18:45 for browser offset -60, got %02d:%02d", gotUTC.Hour(), gotUTC.Minute())
	}
}

func TestEditItemInvalidBuyAfterReturnsValidationAndLeavesItemUnchanged(t *testing.T) {
	app := NewApp()
	now := time.Now()
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 1, Title: "Original", Status: "Waiting", WaitPreset: "24h", PurchaseAllowedAt: now.Add(24 * time.Hour), CreatedAt: now})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("title", "Changed")
	form.Set("wait_preset", "date")
	form.Set("purchase_allowed_at", "not-a-date")

	req := httptest.NewRequest(http.MethodPost, "/items/edit?id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Please enter a valid buy-after date and time.") {
		t.Fatalf("expected buy-after validation error")
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if got := app.items[0].Title; got != "Original" {
		t.Fatalf("expected unchanged item title, got %q", got)
	}
	if got := app.items[0].Status; got != "Waiting" {
		t.Fatalf("expected unchanged status Waiting, got %q", got)
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

func TestReadyToBuySendsSingleNtfyNotification(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	requestCount := 0
	requestBody := ""
	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/impulse-pause" {
			t.Fatalf("expected topic path /impulse-pause, got %s", got)
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	app.mu.Lock()
	app.ntfyURL = ntfyServer.URL
	app.ntfyTopic = "impulse-pause"
	app.dashboardURL = "https://app.example.com"
	app.items = append(app.items, Item{ID: 9, Title: "Laptop stand", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(-time.Minute)})
	app.mu.Unlock()

	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", firstRR.Code)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", secondRR.Code)
	}

	if requestCount != 1 {
		t.Fatalf("expected exactly one ntfy request, got %d", requestCount)
	}
	if !strings.Contains(requestBody, "Dashboard: https://app.example.com/") {
		t.Fatalf("expected dashboard URL in ntfy body, got %q", requestBody)
	}
}

func TestReadyToBuyWithoutNtfyConfigStillPromotesItem(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	app.mu.Lock()
	app.items = append(app.items, Item{ID: 11, Title: "Notebook", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(-time.Minute)})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Ready to buy" {
		t.Fatalf("expected item to be promoted, got %q", app.items[0].Status)
	}
	if !app.items[0].NtfyAttempted {
		t.Fatalf("expected ntfy attempt flag to be set")
	}
}

func TestReadyToBuyContinuesWhenNtfyFails(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	requestCount := 0
	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream issue"))
	}))
	defer ntfyServer.Close()

	app.mu.Lock()
	app.ntfyURL = ntfyServer.URL
	app.ntfyTopic = "impulse-pause"
	app.dashboardURL = "https://app.example.com"
	app.items = append(app.items, Item{ID: 12, Title: "Phone holder", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(-time.Minute)})
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if requestCount != 1 {
		t.Fatalf("expected one ntfy request despite failure, got %d", requestCount)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Ready to buy" {
		t.Fatalf("expected promoted status after ntfy failure, got %q", app.items[0].Status)
	}
}

func TestProfilePersistsNtfySettings(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "30")
	form.Set("ntfy_endpoint", "https://ntfy.sh/")
	form.Set("ntfy_topic", "impulse-pause")

	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	getRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRR, getReq)
	body := getRR.Body.String()
	if !strings.Contains(body, "value=\"https://ntfy.sh\"") {
		t.Fatalf("expected saved ntfy endpoint in profile form")
	}
	if !strings.Contains(body, "value=\"impulse-pause\"") {
		t.Fatalf("expected saved ntfy topic in profile form")
	}
}

func TestProfileRejectsPartialNtfyConfiguration(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "30")
	form.Set("ntfy_endpoint", "https://ntfy.sh")

	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "both ntfy endpoint and topic") {
		t.Fatalf("expected pair validation message, got %q", body)
	}
}

func TestBackgroundPromotionPromotesWithoutHTTPRequest(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{ID: 21, Title: "Cable", Status: "Waiting", PurchaseAllowedAt: time.Now().Add(-time.Minute)})
	app.mu.Unlock()

	app.StartBackgroundPromotion(10 * time.Millisecond)
	time.Sleep(60 * time.Millisecond)

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Ready to buy" {
		t.Fatalf("expected background promotion to update status, got %q", app.items[0].Status)
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

func TestFormatWorkHoursRoundingBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		price      string
		hourlyWage float64
		want       string
	}{
		{name: "rounds down below midpoint", price: "30.4", hourlyWage: 10, want: "3.0"},
		{name: "rounds up at midpoint", price: "30.5", hourlyWage: 10, want: "3.1"},
		{name: "rounds up above midpoint", price: "30.6", hourlyWage: 10, want: "3.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWorkHours(Item{Price: tt.price}, tt.hourlyWage)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
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

func TestInsightsPageShowsDashboardInsights(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Keyboard", Price: "99.99", PriceValue: 99.99, HasPriceValue: true, Tags: "Tech, Desk", Status: "Skipped", PurchaseAllowedAt: time.Now().Add(-time.Hour)},
		Item{ID: 2, Title: "Mouse", Price: "50", PriceValue: 50, HasPriceValue: true, Tags: "tech", Status: "Skipped", PurchaseAllowedAt: time.Now().Add(-time.Hour)},
		Item{ID: 3, Title: "Shoes", Price: "120", PriceValue: 120, HasPriceValue: true, Tags: "Fashion", Status: "Bought", PurchaseAllowedAt: time.Now().Add(-time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/insights", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Skipped items") || !strings.Contains(body, ">2<") {
		t.Fatalf("expected skipped items metric")
	}
	if !strings.Contains(body, "Saved total") || !strings.Contains(body, "149.99") {
		t.Fatalf("expected saved total metric")
	}
	if !strings.Contains(body, "tech · 2") {
		t.Fatalf("expected aggregated top category")
	}
}

func TestInsightsPageShowsZeroStateWhenNoItems(t *testing.T) {
	app := NewApp()

	req := httptest.NewRequest(http.MethodGet, "/insights", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "No data yet. Add items and make decisions to unlock insights.") {
		t.Fatalf("expected dashboard zero state")
	}
}

func TestInsightsMetricsUpdateAfterStatusChange(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                77,
		Title:             "Noise-cancelling headphones",
		Price:             "199",
		PriceValue:        199,
		HasPriceValue:     true,
		Tags:              "Tech",
		Status:            "Ready to buy",
		CreatedAt:         time.Now().Add(-48 * time.Hour),
		PurchaseAllowedAt: time.Now().Add(-24 * time.Hour),
	})
	app.mu.Unlock()

	beforeReq := httptest.NewRequest(http.MethodGet, "/insights", nil)
	beforeRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(beforeRR, beforeReq)

	if beforeRR.Code != http.StatusOK {
		t.Fatalf("expected 200 before status change, got %d", beforeRR.Code)
	}
	beforeBody := beforeRR.Body.String()
	if !strings.Contains(beforeBody, "Skipped items") || !strings.Contains(beforeBody, ">0<") {
		t.Fatalf("expected skipped metric to be zero before status change")
	}
	if !strings.Contains(beforeBody, "Saved total") || !strings.Contains(beforeBody, "0.00") {
		t.Fatalf("expected saved metric to be zero before status change")
	}

	form := url.Values{}
	form.Set("item_id", "77")
	form.Set("status", "Skipped")

	statusReq := httptest.NewRequest(http.MethodPost, "/items/status", strings.NewReader(form.Encode()))
	statusReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	statusRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(statusRR, statusReq)

	if statusRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on status update, got %d", statusRR.Code)
	}

	afterReq := httptest.NewRequest(http.MethodGet, "/insights", nil)
	afterRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(afterRR, afterReq)

	if afterRR.Code != http.StatusOK {
		t.Fatalf("expected 200 after status change, got %d", afterRR.Code)
	}
	afterBody := afterRR.Body.String()
	if !strings.Contains(afterBody, "Skipped items") || !strings.Contains(afterBody, ">1<") {
		t.Fatalf("expected skipped metric to update after status change")
	}
	if !strings.Contains(afterBody, "Saved total") || !strings.Contains(afterBody, "199.00") {
		t.Fatalf("expected saved metric to update after status change")
	}
	if !strings.Contains(afterBody, "tech · 1") {
		t.Fatalf("expected top categories to update after status change")
	}
}

func TestBuildDashboardStatsSortsAndLimitsCategories(t *testing.T) {
	items := []Item{
		{Tags: "gamma"},
		{Tags: "alpha"},
		{Tags: "beta"},
		{Tags: "alpha"},
		{Tags: "delta"},
		{Tags: "beta"},
		{Tags: "beta"},
		{Tags: "delta"},
	}

	_, _, categories := buildDashboardStats(items)

	if len(categories) != 3 {
		t.Fatalf("expected top 3 categories, got %d", len(categories))
	}
	if categories[0].Name != "beta" || categories[0].Count != 3 {
		t.Fatalf("unexpected top category: %+v", categories[0])
	}
	if categories[1].Name != "alpha" || categories[1].Count != 2 {
		t.Fatalf("unexpected second category: %+v", categories[1])
	}
	if categories[2].Name != "delta" || categories[2].Count != 2 {
		t.Fatalf("unexpected third category: %+v", categories[2])
	}
}

func TestBuildMonthlyDecisionTrend(t *testing.T) {
	now := time.Now()
	items := []Item{
		{Status: "Skipped", CreatedAt: time.Date(2026, 1, 4, 12, 0, 0, 0, now.Location())},
		{Status: "Bought", CreatedAt: time.Date(2026, 1, 14, 12, 0, 0, 0, now.Location())},
		{Status: "Skipped", CreatedAt: time.Date(2026, 2, 2, 12, 0, 0, 0, now.Location())},
		{Status: "Waiting", CreatedAt: time.Date(2026, 2, 3, 12, 0, 0, 0, now.Location())},
	}

	trend := buildMonthlyDecisionTrend(items)
	if len(trend) != 2 {
		t.Fatalf("expected 2 months, got %d", len(trend))
	}
	if trend[0].Month != "2026-01" || trend[0].BoughtCount != 1 || trend[0].SkippedCount != 1 {
		t.Fatalf("unexpected first month: %+v", trend[0])
	}
	if trend[1].Month != "2026-02" || trend[1].BoughtCount != 0 || trend[1].SkippedCount != 1 {
		t.Fatalf("unexpected second month: %+v", trend[1])
	}
}

func TestBuildMonthlySavedTrend(t *testing.T) {
	now := time.Now()
	items := []Item{
		{Status: "Skipped", HasPriceValue: true, PriceValue: 40.5, CreatedAt: time.Date(2026, 1, 4, 12, 0, 0, 0, now.Location())},
		{Status: "Skipped", HasPriceValue: true, PriceValue: 9.5, CreatedAt: time.Date(2026, 1, 14, 12, 0, 0, 0, now.Location())},
		{Status: "Skipped", HasPriceValue: false, CreatedAt: time.Date(2026, 2, 2, 12, 0, 0, 0, now.Location())},
		{Status: "Bought", HasPriceValue: true, PriceValue: 100, CreatedAt: time.Date(2026, 2, 3, 12, 0, 0, 0, now.Location())},
	}

	trend := buildMonthlySavedTrend(items)
	if len(trend) != 1 {
		t.Fatalf("expected 1 month, got %d", len(trend))
	}
	if trend[0].Month != "2026-01" || trend[0].Amount != 50 {
		t.Fatalf("unexpected saved trend: %+v", trend[0])
	}
}

func TestBuildCategorySkipRatios(t *testing.T) {
	items := []Item{
		{Status: "Skipped", Tags: "Tech, Home"},
		{Status: "Skipped", Tags: "Tech"},
		{Status: "Bought", Tags: "Tech"},
		{Status: "Bought", Tags: "Home"},
		{Status: "Waiting", Tags: "Tech"},
	}

	ratios := buildCategorySkipRatios(items)
	if len(ratios) != 2 {
		t.Fatalf("expected 2 category ratios, got %d", len(ratios))
	}
	if ratios[0].Name != "tech" || ratios[0].SkippedCount != 2 || ratios[0].DecisionCount != 3 {
		t.Fatalf("unexpected first ratio: %+v", ratios[0])
	}
	if ratios[1].Name != "home" || ratios[1].SkippedCount != 1 || ratios[1].DecisionCount != 2 {
		t.Fatalf("unexpected second ratio: %+v", ratios[1])
	}
}

func TestInsightsTrendSectionsShowZeroStateWithoutDecisions(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Still waiting", Status: "Waiting", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(24 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/insights", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "No monthly decisions yet.") {
		t.Fatalf("expected monthly decision zero state")
	}
	if !strings.Contains(body, "No saved-amount trend yet.") {
		t.Fatalf("expected saved amount zero state")
	}
	if !strings.Contains(body, "No category ratio data yet.") {
		t.Fatalf("expected category ratio zero state")
	}
}

func TestInsightsPageShowsTrendSections(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Keyboard", Price: "99.99", PriceValue: 99.99, HasPriceValue: true, Tags: "Tech", Status: "Skipped", CreatedAt: time.Date(2026, 1, 11, 12, 0, 0, 0, time.Local), PurchaseAllowedAt: time.Now().Add(-time.Hour)},
		Item{ID: 2, Title: "Shoes", Price: "120", PriceValue: 120, HasPriceValue: true, Tags: "Fashion", Status: "Bought", CreatedAt: time.Date(2026, 1, 14, 12, 0, 0, 0, time.Local), PurchaseAllowedAt: time.Now().Add(-time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/insights", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Monthly decision trend") || !strings.Contains(body, "2026-01") {
		t.Fatalf("expected monthly decision trend section")
	}
	if !strings.Contains(body, "Saved amount trend") || !strings.Contains(body, "99.99") {
		t.Fatalf("expected monthly saved trend section")
	}
	if !strings.Contains(body, "Top skip ratios by category") || !strings.Contains(body, "100%") {
		t.Fatalf("expected category ratio section")
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

func TestProfilePersistsDefaultWaitSettings(t *testing.T) {
	app := NewApp()
	form := url.Values{}
	form.Set("hourly_wage", "42.5")
	form.Set("default_wait_preset", "custom")
	form.Set("default_wait_custom_hours", "36")

	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	getRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRR, getReq)
	body := getRR.Body.String()
	if !strings.Contains(body, "<option value=\"custom\" selected") {
		t.Fatalf("expected custom default wait preset selected")
	}
	if !strings.Contains(body, "name=\"default_wait_custom_hours\"") || !strings.Contains(body, "value=\"36\"") {
		t.Fatalf("expected default custom hours to be visible and persisted")
	}
}

func TestItemFormUsesConfiguredDefaultWaitPreset(t *testing.T) {
	app := NewApp()
	app.mu.Lock()
	app.hourlyWage = "25"
	app.defaultWaitPreset = "7d"
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/items/new", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<option value=\"7d\" selected") {
		t.Fatalf("expected configured default wait preset selected in add form")
	}
}

func TestItemsNewShowsOptionalFieldsWithoutDetailsToggle(t *testing.T) {
	app := NewApp()
	seedProfile(app)

	req := httptest.NewRequest(http.MethodGet, "/items/new", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "<details>") {
		t.Fatalf("did not expect optional fields to be hidden behind details")
	}
	if !strings.Contains(body, "name=\"price\"") || !strings.Contains(body, "name=\"link\"") {
		t.Fatalf("expected optional form fields to be directly visible")
	}
}

func TestDeleteItemRemovesItFromHomeAndInsights(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Keep", Status: "Skipped", Price: "12.50", HasPriceValue: true, PriceValue: 12.5, Tags: "Office", PurchaseAllowedAt: time.Now().Add(-time.Hour), CreatedAt: time.Now().Add(-48 * time.Hour)},
		Item{ID: 2, Title: "Delete me", Status: "Skipped", Price: "100.00", HasPriceValue: true, PriceValue: 100, Tags: "Tech", PurchaseAllowedAt: time.Now().Add(-time.Hour), CreatedAt: time.Now().Add(-24 * time.Hour)},
	)
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "2")
	deleteReq := httptest.NewRequest(http.MethodPost, "/items/delete", strings.NewReader(form.Encode()))
	deleteReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	deleteRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", deleteRR.Code)
	}

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("expected home 200, got %d", homeRR.Code)
	}
	if body := homeRR.Body.String(); strings.Contains(body, "Delete me") {
		t.Fatalf("expected deleted item to be absent from home")
	}

	insightsReq := httptest.NewRequest(http.MethodGet, "/insights", nil)
	insightsRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(insightsRR, insightsReq)
	if insightsRR.Code != http.StatusOK {
		t.Fatalf("expected insights 200, got %d", insightsRR.Code)
	}
	body := insightsRR.Body.String()
	if !strings.Contains(body, ">1</p>") {
		t.Fatalf("expected skipped count to reflect remaining item")
	}
	if !strings.Contains(body, "€ 12.50</p>") {
		t.Fatalf("expected saved total to exclude deleted item")
	}
}

func TestSnoozeItemMovesReadyToBuyBackToWaiting(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	start := time.Now().Add(-2 * time.Hour)

	app.mu.Lock()
	app.items = append(app.items, Item{
		ID:                9,
		Title:             "Tablet",
		Status:            "Ready to buy",
		CreatedAt:         start,
		PurchaseAllowedAt: start,
	})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "9")
	form.Set("snooze_preset", "24h")

	req := httptest.NewRequest(http.MethodPost, "/items/snooze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.items[0].Status != "Waiting" {
		t.Fatalf("expected status Waiting after snooze, got %q", app.items[0].Status)
	}
	if !app.items[0].PurchaseAllowedAt.After(time.Now().Add(23 * time.Hour)) {
		t.Fatalf("expected purchase allowed timestamp to be pushed into future")
	}
}

func TestSnoozeItemRejectsFinalStatus(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 10, Title: "Final", Status: "Skipped", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(-time.Hour)})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "10")
	form.Set("snooze_preset", "24h")

	req := httptest.NewRequest(http.MethodPost, "/items/snooze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestHomeShowsSnoozeOnlyForReadyItems(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Waiting item", Status: "Waiting", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(24 * time.Hour)},
		Item{ID: 2, Title: "Ready item", Status: "Ready to buy", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(-24 * time.Hour)},
		Item{ID: 3, Title: "Final item", Status: "Bought", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(-24 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Snooze +24h") {
		t.Fatalf("expected snooze controls to render for ready item")
	}
	if strings.Count(body, "Snooze +24h") != 1 {
		t.Fatalf("expected snooze controls to render only once for ready item")
	}
}

func TestSnoozeItemRejectsWaitingStatus(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 11, Title: "Waiting", Status: "Waiting", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(time.Hour)})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "11")
	form.Set("snooze_preset", "24h")

	req := httptest.NewRequest(http.MethodPost, "/items/snooze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSnoozeItemRejectsInvalidPreset(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	app.mu.Lock()
	app.items = append(app.items, Item{ID: 12, Title: "Ready", Status: "Ready to buy", CreatedAt: time.Now(), PurchaseAllowedAt: time.Now().Add(-time.Hour)})
	app.mu.Unlock()

	form := url.Values{}
	form.Set("item_id", "12")
	form.Set("snooze_preset", "7d")

	req := httptest.NewRequest(http.MethodPost, "/items/snooze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestSnoozeRequiresPost(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/items/snooze", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
func TestDeleteItemRequiresPost(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest(http.MethodGet, "/items/delete", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestProfileCurrencyDefaultsToEuro(t *testing.T) {
	app := NewApp()

	req := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "id=\"currency\"") || !strings.Contains(body, "value=\"€\"") {
		t.Fatalf("expected default euro currency in profile form")
	}
}

func TestProfileCurrencyPersistsAndRendersAcrossViews(t *testing.T) {
	app := NewApp()

	form := url.Values{}
	form.Set("hourly_wage", "30")
	form.Set("currency", "CHF")
	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	itemForm := url.Values{}
	itemForm.Set("title", "Monitor")
	itemForm.Set("price", "199.90")
	itemReq := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(itemForm.Encode()))
	itemReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	itemRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(itemRR, itemReq)
	if itemRR.Code != http.StatusSeeOther {
		t.Fatalf("expected item create redirect, got %d", itemRR.Code)
	}

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("expected home 200, got %d", homeRR.Code)
	}
	if body := homeRR.Body.String(); !strings.Contains(body, "CHF 199.90") {
		t.Fatalf("expected dashboard item price to include currency")
	}

	newReq := httptest.NewRequest(http.MethodGet, "/items/new", nil)
	newRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(newRR, newReq)
	if newRR.Code != http.StatusOK {
		t.Fatalf("expected new item form 200, got %d", newRR.Code)
	}
	if body := newRR.Body.String(); !strings.Contains(body, "Currency: CHF") {
		t.Fatalf("expected item form to include profile currency")
	}

	app.mu.Lock()
	app.items[0].Status = "Skipped"
	app.items[0].HasPriceValue = true
	app.items[0].PriceValue = 199.9
	app.mu.Unlock()

	insightsReq := httptest.NewRequest(http.MethodGet, "/insights", nil)
	insightsRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(insightsRR, insightsReq)
	if insightsRR.Code != http.StatusOK {
		t.Fatalf("expected insights 200, got %d", insightsRR.Code)
	}
	if body := insightsRR.Body.String(); !strings.Contains(body, "CHF 199.90") {
		t.Fatalf("expected insights saved total to include currency")
	}
}

func TestProfileCurrencyFallsBackToEuroWhenEmpty(t *testing.T) {
	app := NewApp()

	form := url.Values{}
	form.Set("hourly_wage", "30")
	form.Set("currency", "")
	req := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	profileRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(profileRR, profileReq)
	if profileRR.Code != http.StatusOK {
		t.Fatalf("expected profile 200, got %d", profileRR.Code)
	}
	if body := profileRR.Body.String(); !strings.Contains(body, "value=\"€\"") {
		t.Fatalf("expected empty currency to fallback to euro")
	}
}

func TestHomeDefaultsToOpenStatuses(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	now := time.Now()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "Waiting item", Status: "Waiting", CreatedAt: now.Add(-4 * time.Hour), PurchaseAllowedAt: now.Add(2 * time.Hour)},
		Item{ID: 2, Title: "Ready item", Status: "Ready to buy", CreatedAt: now.Add(-3 * time.Hour), PurchaseAllowedAt: now.Add(-2 * time.Hour)},
		Item{ID: 3, Title: "Bought item", Status: "Bought", CreatedAt: now.Add(-2 * time.Hour), PurchaseAllowedAt: now.Add(-4 * time.Hour)},
		Item{ID: 4, Title: "Skipped item", Status: "Skipped", CreatedAt: now.Add(-1 * time.Hour), PurchaseAllowedAt: now.Add(-5 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Waiting item") || !strings.Contains(body, "Ready item") {
		t.Fatalf("expected waiting and ready items in default list")
	}
	if strings.Contains(body, "Bought item") || strings.Contains(body, "Skipped item") {
		t.Fatalf("did not expect closed items in default list")
	}
}

func TestHomeNextReadySortAcrossStatusesWhenAllSelected(t *testing.T) {
	app := NewApp()
	seedProfile(app)
	now := time.Now()

	app.mu.Lock()
	app.items = append(app.items,
		Item{ID: 1, Title: "ready-early", Status: "Ready to buy", CreatedAt: now.Add(-6 * time.Hour), PurchaseAllowedAt: now.Add(-3 * time.Hour)},
		Item{ID: 2, Title: "ready-late", Status: "Ready to buy", CreatedAt: now.Add(-5 * time.Hour), PurchaseAllowedAt: now.Add(-1 * time.Hour)},
		Item{ID: 3, Title: "waiting-early", Status: "Waiting", CreatedAt: now.Add(-4 * time.Hour), PurchaseAllowedAt: now.Add(1 * time.Hour)},
		Item{ID: 4, Title: "waiting-late", Status: "Waiting", CreatedAt: now.Add(-3 * time.Hour), PurchaseAllowedAt: now.Add(5 * time.Hour)},
		Item{ID: 5, Title: "bought-new", Status: "Bought", CreatedAt: now.Add(-30 * time.Minute), PurchaseAllowedAt: now.Add(-10 * time.Hour)},
		Item{ID: 6, Title: "skipped-old", Status: "Skipped", CreatedAt: now.Add(-2 * time.Hour), PurchaseAllowedAt: now.Add(-11 * time.Hour)},
	)
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/?sort=next_ready&status=Waiting&status=Ready+to+buy&status=Bought&status=Skipped", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()

	ordered := []string{"ready-early", "ready-late", "waiting-early", "waiting-late", "bought-new", "skipped-old"}
	prev := -1
	for _, label := range ordered {
		idx := strings.Index(body, label)
		if idx == -1 {
			t.Fatalf("expected %s in response", label)
		}
		if idx < prev {
			t.Fatalf("expected %s to appear after previous item", label)
		}
		prev = idx
	}
}

func TestFilterAndSortItemsMonkeyishNextReadyOrdering(t *testing.T) {
	now := time.Now()
	statuses := []string{"Waiting", "Ready to buy", "Bought", "Skipped"}
	items := make([]Item, 0, 60)
	for i := 0; i < 60; i++ {
		status := statuses[i%len(statuses)]
		items = append(items, Item{
			ID:                i + 1,
			Title:             "item",
			Status:            status,
			CreatedAt:         now.Add(-time.Duration((i*37)%500) * time.Minute),
			PurchaseAllowedAt: now.Add(time.Duration((i*53)%400-200) * time.Minute),
		})
	}

	sorted := filterAndSortItems(items, "", statuses, "", "next_ready")
	if len(sorted) != len(items) {
		t.Fatalf("expected %d items, got %d", len(items), len(sorted))
	}

	rank := func(status string) int {
		switch status {
		case "Ready to buy":
			return 0
		case "Waiting":
			return 1
		default:
			return 2
		}
	}

	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]
		if rank(prev.Status) > rank(curr.Status) {
			t.Fatalf("status rank regression from %s to %s at %d", prev.Status, curr.Status, i)
		}

		if rank(prev.Status) == rank(curr.Status) {
			switch prev.Status {
			case "Ready to buy", "Waiting":
				if prev.PurchaseAllowedAt.After(curr.PurchaseAllowedAt) {
					t.Fatalf("purchase order regression within %s at %d", prev.Status, i)
				}
			case "Bought", "Skipped":
				if prev.CreatedAt.Before(curr.CreatedAt) {
					t.Fatalf("created order regression within closed statuses at %d", i)
				}
			}
		}
	}
}
