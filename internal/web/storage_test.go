package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppWithSQLiteCreatesSchemaAndPersistsData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")

	app, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app to initialize with sqlite, got error: %v", err)
	}

	profileForm := url.Values{}
	profileForm.Set("hourly_wage", "35")
	profileForm.Set("default_wait_preset", "7d")
	profileForm.Set("currency", "EUR")
	profileReq := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(profileForm.Encode()))
	profileReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	profileRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(profileRR, profileReq)
	if profileRR.Code != http.StatusSeeOther {
		t.Fatalf("expected profile save redirect, got %d", profileRR.Code)
	}

	itemForm := url.Values{}
	itemForm.Set("title", "Bike light")
	itemReq := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(itemForm.Encode()))
	itemReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	itemRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(itemRR, itemReq)
	if itemRR.Code != http.StatusSeeOther {
		t.Fatalf("expected item save redirect, got %d", itemRR.Code)
	}

	reloadedApp, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app reload with sqlite, got error: %v", err)
	}

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	reloadedApp.Handler().ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("expected home 200 after reload, got %d", homeRR.Code)
	}
	if body := homeRR.Body.String(); !strings.Contains(body, "Bike light") {
		t.Fatalf("expected persisted item after reload")
	}

	settingsReq := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	settingsRR := httptest.NewRecorder()
	reloadedApp.Handler().ServeHTTP(settingsRR, settingsReq)
	if settingsRR.Code != http.StatusOK {
		t.Fatalf("expected profile settings 200 after reload, got %d", settingsRR.Code)
	}
	if body := settingsRR.Body.String(); !strings.Contains(body, "value=\"35\"") {
		t.Fatalf("expected persisted profile hourly wage after reload")
	}
	if body := settingsRR.Body.String(); !strings.Contains(body, "value=\"EUR\"") {
		t.Fatalf("expected persisted profile currency after reload")
	}
}

func TestDeleteItemPersistsInSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")

	app, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app to initialize with sqlite, got error: %v", err)
	}

	profileForm := url.Values{}
	profileForm.Set("hourly_wage", "35")
	profileReq := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(profileForm.Encode()))
	profileReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	profileRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(profileRR, profileReq)
	if profileRR.Code != http.StatusSeeOther {
		t.Fatalf("expected profile save redirect, got %d", profileRR.Code)
	}

	itemForm := url.Values{}
	itemForm.Set("title", "Delete me")
	itemReq := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(itemForm.Encode()))
	itemReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	itemRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(itemRR, itemReq)
	if itemRR.Code != http.StatusSeeOther {
		t.Fatalf("expected item save redirect, got %d", itemRR.Code)
	}

	deleteForm := url.Values{}
	deleteForm.Set("item_id", "1")
	deleteReq := httptest.NewRequest(http.MethodPost, "/items/delete", strings.NewReader(deleteForm.Encode()))
	deleteReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	deleteRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusSeeOther {
		t.Fatalf("expected delete redirect, got %d", deleteRR.Code)
	}

	reloadedApp, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app reload with sqlite, got error: %v", err)
	}

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	reloadedApp.Handler().ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("expected home 200 after reload, got %d", homeRR.Code)
	}
	if body := homeRR.Body.String(); strings.Contains(body, "Delete me") {
		t.Fatalf("expected deleted item to stay deleted after reload")
	}
}

func TestRenameProfilePersistsAcrossReloadInSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")

	app, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app to initialize with sqlite, got error: %v", err)
	}

	setupForm := url.Values{}
	setupForm.Set("profile_name", "OldName")
	setupForm.Set("hourly_wage", "42")
	setupForm.Set("default_wait_preset", "24h")
	setupReq := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(setupForm.Encode()))
	setupReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setupRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(setupRR, setupReq)
	if setupRR.Code != http.StatusSeeOther {
		t.Fatalf("expected profile save redirect, got %d", setupRR.Code)
	}

	itemForm := url.Values{}
	itemForm.Set("title", "Rename persists")
	itemReq := httptest.NewRequest(http.MethodPost, "/items/new", strings.NewReader(itemForm.Encode()))
	itemReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	itemRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(itemRR, itemReq)
	if itemRR.Code != http.StatusSeeOther {
		t.Fatalf("expected item save redirect, got %d", itemRR.Code)
	}

	renameForm := url.Values{}
	renameForm.Set("profile_name", "NewName")
	renameForm.Set("hourly_wage", "42")
	renameForm.Set("default_wait_preset", "24h")
	renameReq := httptest.NewRequest(http.MethodPost, "/settings/profile", strings.NewReader(renameForm.Encode()))
	renameReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	renameRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(renameRR, renameReq)
	if renameRR.Code != http.StatusSeeOther {
		t.Fatalf("expected rename redirect, got %d", renameRR.Code)
	}

	reloadedApp, err := NewAppWithSQLite(dbPath)
	if err != nil {
		t.Fatalf("expected app reload with sqlite, got error: %v", err)
	}

	switchReq := httptest.NewRequest(http.MethodPost, "/switch-profile", strings.NewReader(url.Values{"profile_name": {"NewName"}}.Encode()))
	switchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	switchRR := httptest.NewRecorder()
	reloadedApp.Handler().ServeHTTP(switchRR, switchReq)
	if switchRR.Code != http.StatusSeeOther {
		t.Fatalf("expected switch redirect to renamed profile, got %d", switchRR.Code)
	}

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	reloadedApp.Handler().ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("expected home 200 after reload, got %d", homeRR.Code)
	}
	if body := homeRR.Body.String(); !strings.Contains(body, "Rename persists") {
		t.Fatalf("expected item to remain visible under renamed profile")
	}
}
