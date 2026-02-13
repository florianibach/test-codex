package web

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html assets/*.css
var embeddedFiles embed.FS

type Item struct {
	ID                int
	Title             string
	Price             string
	PriceValue        float64
	HasPriceValue     bool
	Link              string
	Note              string
	Tags              string
	Status            string
	WaitPreset        string
	WaitCustomHours   string
	PurchaseAllowedAt time.Time
	CreatedAt         time.Time
	NtfyAttempted     bool
}

type homeViewData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	Items           []Item
	SearchQuery     string
	SelectedStatus  map[string]bool
	TagFilter       string
	TagOptions      []string
	SortBy          string
	HasActiveFilter bool
	TotalItems      int
	HourlyWage      float64
	HasHourlyWage   bool
	Currency        string
	ActiveProfile   string
}

type insightsViewData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	ItemCount       int
	SkippedCount    int
	SavedAmount     float64
	TopCategories   []categoryCount
	DecisionTrend   []monthlyDecisionTrend
	SavedTrend      []monthlySavedAmount
	CategoryRatios  []categorySkipRatio
	Currency        string
	ActiveProfile   string
}

type categoryCount struct {
	Name  string
	Count int
}

type monthlyDecisionTrend struct {
	Month        string
	BoughtCount  int
	SkippedCount int
}

type monthlySavedAmount struct {
	Month  string
	Amount float64
}

type categorySkipRatio struct {
	Name          string
	SkippedCount  int
	DecisionCount int
	Ratio         float64
}

type itemFormViewData struct {
	Title                string
	CurrentPath          string
	ContentTemplate      string
	ScriptTemplate       string
	Items                []Item
	ItemID               int
	FormAction           string
	SubmitLabel          string
	CancelHref           string
	FormValues           Item
	TagOptions           []string
	SelectedTags         map[string]bool
	PurchaseAllowedInput string
	Error                string
	Currency             string
	ActiveProfile        string
}

var defaultTagOptions = []string{"Tech", "Audio", "Gaming", "Home", "Fashion", "Sports", "Office", "Travel", "Health", "Education"}

type profileViewData struct {
	Title                  string
	CurrentPath            string
	ContentTemplate        string
	ScriptTemplate         string
	ProfileName            string
	ProfileHourly          string
	DefaultWaitPreset      string
	DefaultWaitCustomHours string
	NtfyEndpoint           string
	NtfyTopic              string
	Currency               string
	ProfileError           string
	ProfileFeedback        string
	ActiveProfile          string
}

type pageData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	ActiveProfile   string
}

type profileSwitchViewData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	SelectedName    string
	Names           []string
	Error           string
	ActiveProfile   string
}

type App struct {
	templates              *template.Template
	mux                    *http.ServeMux
	db                     *sql.DB
	mu                     sync.RWMutex
	items                  []Item
	hourlyWage             string
	defaultWaitPreset      string
	defaultWaitCustomHours string
	ntfyURL                string
	ntfyTopic              string
	currency               string
	dashboardURL           string
	nextID                 int
	activeUserID           string
	profileExists          bool
}

func NewApp() *App {
	app, err := newAppWithDB(nil)
	if err != nil {
		panic(err)
	}
	return app
}

func NewAppWithSQLite(dbPath string) (*App, error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return nil, err
	}

	app, err := newAppWithDB(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return app, nil
}

func newAppWithDB(db *sql.DB) (*App, error) {
	tpls := template.Must(template.New("").Funcs(template.FuncMap{
		"statusBadgeClass":   statusBadgeClass,
		"workHoursAvailable": workHoursAvailable,
		"formatWorkHours":    formatWorkHours,
		"formatMoney":        formatMoney,
		"mul100":             mul100,
	}).ParseFS(embeddedFiles, "templates/*.html"))
	mux := http.NewServeMux()

	app := &App{templates: tpls, mux: mux, db: db, nextID: 1, activeUserID: defaultUserID}
	if err := app.loadStateFromDB(app.activeUserID); err != nil {
		return nil, err
	}
	app.routes()
	app.StartBackgroundPromotion(5 * time.Second)

	return app, nil
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.home)
	a.mux.HandleFunc("/switch-profile", a.switchProfile)
	a.mux.HandleFunc("/items/new", a.itemForm)
	a.mux.HandleFunc("/items/edit", a.editItemForm)
	a.mux.HandleFunc("/items/delete", a.deleteItem)
	a.mux.HandleFunc("/items/snooze", a.snoozeItem)
	a.mux.HandleFunc("/insights", a.insights)
	a.mux.HandleFunc("/settings/profile", a.profileSettings)
	a.mux.HandleFunc("/settings/profile/delete", a.deleteProfile)
	a.mux.HandleFunc("/profile", a.legacyProfile)
	a.mux.HandleFunc("/items/status", a.updateItemStatus)
	a.mux.HandleFunc("/healthz", a.health)
	a.mux.HandleFunc("/about", a.about)
	a.mux.Handle("/assets/", http.FileServer(http.FS(embeddedFiles)))
}

func (a *App) Handler() http.Handler {
	return loggingMiddleware(a.mux)
}

func (a *App) StartBackgroundPromotion(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	go func() {
		promote := func() {
			a.mu.Lock()
			a.promoteReadyItemsLocked(time.Now())
			a.mu.Unlock()
		}

		promote()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			promote()
		}
	}()
}

func (a *App) SetDashboardURL(raw string) {
	a.mu.Lock()
	a.dashboardURL = strings.TrimRight(strings.TrimSpace(raw), "/")
	a.mu.Unlock()
}

func (a *App) activateProfileFromRequest(r *http.Request) error {
	cookie, err := r.Cookie("active_profile")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil
		}
		return err
	}
	name := strings.TrimSpace(cookie.Value)
	if name == "" {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeUserID == name {
		return nil
	}
	a.activeUserID = name
	return a.loadStateFromDB(name)
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if err := a.activateProfileFromRequest(r); err != nil {
			http.Error(w, "could not activate profile", http.StatusInternalServerError)
			return
		}
		if !a.hasActiveProfile() {
			http.Redirect(w, r, "/switch-profile", http.StatusSeeOther)
			return
		}
		if !a.hasProfile() {
			http.Redirect(w, r, "/settings/profile", http.StatusSeeOther)
			return
		}
		a.renderHome(w, r, homeViewData{Title: "Impulse Pause", CurrentPath: "/"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) insights(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/insights" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.renderInsights(w, insightsViewData{Title: "Insights", CurrentPath: "/insights"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) itemForm(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.renderItemForm(w, itemFormViewData{Title: "Add item", CurrentPath: "/items/new"})
	case http.MethodPost:
		a.createItem(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) editItemForm(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.renderEditItemForm(w, r, itemFormViewData{Title: "Edit item", CurrentPath: "/"})
	case http.MethodPost:
		a.updateItem(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) createItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	item := Item{
		Title:           strings.TrimSpace(r.FormValue("title")),
		Price:           strings.TrimSpace(r.FormValue("price")),
		Link:            strings.TrimSpace(r.FormValue("link")),
		Note:            strings.TrimSpace(r.FormValue("note")),
		Tags:            parseTagsFromForm(r.Form["tags"], r.FormValue("custom_tag")),
		WaitPreset:      strings.TrimSpace(r.FormValue("wait_preset")),
		WaitCustomHours: strings.TrimSpace(r.FormValue("wait_custom_hours")),
	}

	if item.WaitPreset == "" {
		a.mu.RLock()
		item.WaitPreset = defaultWaitPreset(a.defaultWaitPreset)
		if item.WaitPreset == "custom" {
			item.WaitCustomHours = a.defaultWaitCustomHours
		}
		a.mu.RUnlock()
	}

	if parsedPrice, ok := parsePrice(item.Price); ok {
		item.PriceValue = parsedPrice
		item.HasPriceValue = true
	}

	if item.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		a.renderItemForm(w, itemFormViewData{
			Title:       "Add item",
			CurrentPath: "/items/new",
			FormValues:  item,
			Error:       "Please enter a title.",
		})
		return
	}

	now := time.Now()
	purchaseAllowedInput := strings.TrimSpace(r.FormValue("purchase_allowed_at"))
	timezoneOffsetMinutes := strings.TrimSpace(r.FormValue("timezone_offset_minutes"))
	purchaseAllowedAt, err := resolvePurchaseAllowedAt(item.WaitPreset, item.WaitCustomHours, purchaseAllowedInput, timezoneOffsetMinutes, now)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderItemForm(w, itemFormViewData{
			Title:                "Add item",
			CurrentPath:          "/items/new",
			FormValues:           item,
			PurchaseAllowedInput: purchaseAllowedInput,
			Error:                err.Error(),
		})
		return
	}

	item.Status = activeStatusForPurchaseAllowedAt(purchaseAllowedAt, now)
	item.WaitPreset = normalizeItemWaitPreset(item.WaitPreset)
	item.CreatedAt = now
	item.PurchaseAllowedAt = purchaseAllowedAt

	a.mu.Lock()
	if err := a.insertItemLocked(&item); err != nil {
		a.mu.Unlock()
		log.Printf("db error while creating item: %v", err)
		http.Error(w, "could not save item", http.StatusInternalServerError)
		return
	}
	a.items = append([]Item{item}, a.items...)
	a.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) renderEditItemForm(w http.ResponseWriter, r *http.Request, data itemFormViewData) {
	id, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	if data.FormValues.ID == 0 {
		for i := range a.items {
			if a.items[i].ID == id {
				data.FormValues = a.items[i]
				break
			}
		}
	}
	a.mu.RUnlock()

	if data.FormValues.ID == 0 {
		http.NotFound(w, r)
		return
	}

	data.ItemID = id
	data.FormAction = "/items/edit?id=" + strconv.Itoa(id)
	data.SubmitLabel = "Save changes"
	data.CancelHref = "/"
	a.renderItemForm(w, data)
}

func (a *App) updateItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	item := Item{
		ID:              id,
		Title:           strings.TrimSpace(r.FormValue("title")),
		Price:           strings.TrimSpace(r.FormValue("price")),
		Link:            strings.TrimSpace(r.FormValue("link")),
		Note:            strings.TrimSpace(r.FormValue("note")),
		Tags:            parseTagsFromForm(r.Form["tags"], r.FormValue("custom_tag")),
		WaitPreset:      strings.TrimSpace(r.FormValue("wait_preset")),
		WaitCustomHours: strings.TrimSpace(r.FormValue("wait_custom_hours")),
	}

	if parsedPrice, ok := parsePrice(item.Price); ok {
		item.PriceValue = parsedPrice
		item.HasPriceValue = true
	}

	if item.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		a.renderEditItemForm(w, r, itemFormViewData{
			Title:       "Edit item",
			CurrentPath: "/",
			FormValues:  item,
			Error:       "Please enter a title.",
		})
		return
	}

	now := time.Now()
	purchaseAllowedInput := strings.TrimSpace(r.FormValue("purchase_allowed_at"))
	timezoneOffsetMinutes := strings.TrimSpace(r.FormValue("timezone_offset_minutes"))
	purchaseAllowedAt, err := resolvePurchaseAllowedAt(item.WaitPreset, item.WaitCustomHours, purchaseAllowedInput, timezoneOffsetMinutes, now)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderEditItemForm(w, r, itemFormViewData{
			Title:                "Edit item",
			CurrentPath:          "/",
			FormValues:           item,
			PurchaseAllowedInput: purchaseAllowedInput,
			Error:                err.Error(),
		})
		return
	}

	item.WaitPreset = normalizeItemWaitPreset(item.WaitPreset)

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.items {
		if a.items[i].ID != id {
			continue
		}

		existing := a.items[i]
		item.CreatedAt = existing.CreatedAt
		item.NtfyAttempted = existing.NtfyAttempted

		item.PurchaseAllowedAt = purchaseAllowedAt
		if existing.Status == "Bought" {
			item.Status = "Bought"
		} else {
			item.Status = activeStatusForPurchaseAllowedAt(purchaseAllowedAt, now)
			if item.Status == "Waiting" {
				item.NtfyAttempted = false
			}
		}

		a.items[i] = item
		if err := a.updateItemLocked(item); err != nil {
			log.Printf("db error while updating item: %v", err)
			http.Error(w, "could not update item", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func (a *App) profileSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.renderProfile(w, profileViewData{
			Title:           "Profile settings",
			CurrentPath:     "/settings/profile",
			ProfileFeedback: feedbackFromQuery(r),
		})
	case http.MethodPost:
		a.saveProfile(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) deleteProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	names, err := a.listProfileNames()
	if err != nil {
		http.Error(w, "could not load profiles", http.StatusInternalServerError)
		return
	}
	if len(names) <= 1 {
		w.WriteHeader(http.StatusConflict)
		a.renderProfile(w, profileViewData{
			Title:        "Profile settings",
			CurrentPath:  "/settings/profile",
			ProfileError: "The last remaining profile cannot be deleted. Please create or switch to another profile first.",
		})
		return
	}

	a.mu.Lock()
	profileName := a.currentUserIDLocked()
	if err := a.deleteProfileLocked(profileName); err != nil {
		a.mu.Unlock()
		log.Printf("db error while deleting profile: %v", err)
		http.Error(w, "could not delete profile", http.StatusInternalServerError)
		return
	}
	a.activeUserID = ""
	a.items = nil
	a.hourlyWage = ""
	a.defaultWaitPreset = defaultWaitPreset("")
	a.defaultWaitCustomHours = ""
	a.ntfyURL = ""
	a.ntfyTopic = ""
	a.currency = ""
	a.profileExists = false
	a.nextID = 1
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: "active_profile", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.Redirect(w, r, "/switch-profile", http.StatusSeeOther)
}

func (a *App) legacyProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/settings/profile", http.StatusSeeOther)
		return
	}
	a.saveProfile(w, r)
}

func feedbackFromQuery(r *http.Request) string {
	if r.URL.Query().Get("saved") == "1" {
		return "Profile saved."
	}
	return ""
}

func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	profileNameRaw := strings.TrimSpace(r.FormValue("profile_name"))
	if profileNameRaw == "" {
		profileNameRaw = a.activeProfileName()
	}
	profileName, err := parseProfileName(profileNameRaw)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileName:            strings.TrimSpace(profileNameRaw),
			ProfileHourly:          strings.TrimSpace(r.FormValue("hourly_wage")),
			DefaultWaitPreset:      strings.TrimSpace(r.FormValue("default_wait_preset")),
			DefaultWaitCustomHours: strings.TrimSpace(r.FormValue("default_wait_custom_hours")),
			NtfyEndpoint:           strings.TrimRight(strings.TrimSpace(r.FormValue("ntfy_endpoint")), "/"),
			NtfyTopic:              strings.TrimSpace(r.FormValue("ntfy_topic")),
			Currency:               normalizeCurrency(r.FormValue("currency")),
			ProfileError:           err.Error(),
		})
		return
	}

	hourlyWage := strings.TrimSpace(r.FormValue("hourly_wage"))
	defaultPreset := strings.TrimSpace(r.FormValue("default_wait_preset"))
	defaultCustomHours := strings.TrimSpace(r.FormValue("default_wait_custom_hours"))
	ntfyURL := strings.TrimRight(strings.TrimSpace(r.FormValue("ntfy_endpoint")), "/")
	ntfyTopic := strings.TrimSpace(r.FormValue("ntfy_topic"))
	currency := normalizeCurrency(r.FormValue("currency"))

	if _, err := parseHourlyWage(hourlyWage); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileName:            profileName,
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			Currency:               currency,
			ProfileError:           err.Error(),
		})
		return
	}

	if _, err := parseWaitDuration(defaultPreset, defaultCustomHours); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileName:            profileName,
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			Currency:               currency,
			ProfileError:           err.Error(),
		})
		return
	}

	if (ntfyURL == "" && ntfyTopic != "") || (ntfyURL != "" && ntfyTopic == "") {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileName:            profileName,
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			Currency:               currency,
			ProfileError:           "Please provide both ntfy endpoint and topic, or leave both empty.",
		})
		return
	}

	a.mu.Lock()
	previousProfileName := a.currentUserIDLocked()
	if profileName != previousProfileName {
		if err := a.renameProfileLocked(previousProfileName, profileName); err != nil {
			a.mu.Unlock()
			log.Printf("db error while renaming profile: %v", err)
			http.Error(w, "could not rename profile", http.StatusInternalServerError)
			return
		}
		a.activeUserID = profileName
	}
	a.hourlyWage = hourlyWage
	a.defaultWaitPreset = defaultWaitPreset(defaultPreset)
	if a.defaultWaitPreset == "custom" {
		a.defaultWaitCustomHours = defaultCustomHours
	} else {
		a.defaultWaitCustomHours = ""
	}
	a.ntfyURL = ntfyURL
	a.ntfyTopic = ntfyTopic
	a.currency = currency
	if err := a.persistProfileLocked(); err != nil {
		a.mu.Unlock()
		log.Printf("db error while saving profile: %v", err)
		http.Error(w, "could not save profile", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "active_profile", Value: profileName, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})

	http.Redirect(w, r, "/settings/profile?saved=1", http.StatusSeeOther)
}

func (a *App) updateItemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("item_id")))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	newStatus := strings.TrimSpace(r.FormValue("status"))
	if !slices.Contains([]string{"Bought", "Skipped"}, newStatus) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.promoteReadyItemsLocked(time.Now())

	for i := range a.items {
		if a.items[i].ID != id {
			continue
		}

		if a.items[i].Status != "Ready to buy" {
			http.Error(w, "status transition not allowed", http.StatusConflict)
			return
		}

		a.items[i].Status = newStatus
		if err := a.updateItemStatusLocked(id, newStatus); err != nil {
			log.Printf("db error while updating item status: %v", err)
			http.Error(w, "could not update item status", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func (a *App) deleteItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("item_id")))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.items {
		if a.items[i].ID != id {
			continue
		}

		a.items = append(a.items[:i], a.items[i+1:]...)
		if err := a.deleteItemLocked(id); err != nil {
			log.Printf("db error while deleting item: %v", err)
			http.Error(w, "could not delete item", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func (a *App) snoozeItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("item_id")))
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	snoozePreset := strings.TrimSpace(r.FormValue("snooze_preset"))
	if snoozePreset != "24h" {
		http.Error(w, "invalid snooze preset", http.StatusBadRequest)
		return
	}

	duration := 24 * time.Hour

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	a.promoteReadyItemsLocked(now)

	for i := range a.items {
		if a.items[i].ID != id {
			continue
		}

		if a.items[i].Status != "Ready to buy" {
			http.Error(w, "snooze is only allowed for ready items", http.StatusConflict)
			return
		}

		base := a.items[i].PurchaseAllowedAt
		if base.Before(now) {
			base = now
		}

		a.items[i].PurchaseAllowedAt = base.Add(duration)
		a.items[i].Status = "Waiting"
		a.items[i].NtfyAttempted = false

		if err := a.updateItemLocked(a.items[i]); err != nil {
			log.Printf("db error while snoozing item: %v", err)
			http.Error(w, "could not snooze item", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func parsePurchaseAllowedAt(raw string, timezoneOffsetMinutesRaw string) (time.Time, error) {
	location := time.Local
	if timezoneOffsetMinutesRaw != "" {
		offsetMinutes, err := strconv.Atoi(timezoneOffsetMinutesRaw)
		if err != nil {
			return time.Time{}, errors.New("Please enter a valid buy-after date and time.")
		}
		location = time.FixedZone("browser", -offsetMinutes*60)
	}

	parsed, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(raw), location)
	if err != nil {
		return time.Time{}, errors.New("Please enter a valid buy-after date and time.")
	}
	return parsed, nil
}

func resolvePurchaseAllowedAt(waitPreset string, waitCustomHours string, purchaseAllowedRaw string, timezoneOffsetMinutesRaw string, now time.Time) (time.Time, error) {
	if normalizeItemWaitPreset(waitPreset) == "date" {
		if strings.TrimSpace(purchaseAllowedRaw) == "" {
			return time.Time{}, errors.New("Please enter a buy-after date and time.")
		}
		return parsePurchaseAllowedAt(purchaseAllowedRaw, strings.TrimSpace(timezoneOffsetMinutesRaw))
	}

	waitDuration, err := parseWaitDuration(waitPreset, waitCustomHours)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(waitDuration), nil
}

func activeStatusForPurchaseAllowedAt(purchaseAllowedAt, now time.Time) string {
	if purchaseAllowedAt.After(now) {
		return "Waiting"
	}
	return "Ready to buy"
}

func parseWaitDuration(waitPreset string, waitCustomHours string) (time.Duration, error) {
	preset := strings.TrimSpace(waitPreset)
	if preset == "" {
		preset = "24h"
	}

	switch preset {
	case "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	case "custom":
		hours, err := strconv.ParseFloat(strings.TrimSpace(waitCustomHours), 64)
		if err != nil || hours <= 0 {
			return 0, errors.New("Please enter a valid number of custom hours (> 0).")
		}
		return time.Duration(hours * float64(time.Hour)), nil
	default:
		return 0, errors.New("Please select a valid wait time.")
	}
}

func normalizeItemWaitPreset(raw string) string {
	switch strings.TrimSpace(raw) {
	case "7d", "30d", "custom", "date":
		return strings.TrimSpace(raw)
	default:
		return "24h"
	}
}

func defaultWaitPreset(raw string) string {
	return normalizeItemWaitPreset(raw)
}

func (a *App) hasProfile() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.db == nil {
		return a.profileExists || strings.TrimSpace(a.hourlyWage) != ""
	}
	return a.profileExists
}

func normalizeSortBy(raw string) string {
	switch strings.TrimSpace(raw) {
	case "newest", "oldest", "price_asc", "price_desc":
		return strings.TrimSpace(raw)
	default:
		return "next_ready"
	}
}

const defaultProfileHourlyWage = "25"

var allStatuses = []string{"Waiting", "Ready to buy", "Bought", "Skipped"}

func parseStatusFilter(raw []string) ([]string, bool) {
	if len(raw) == 0 {
		return []string{"Waiting", "Ready to buy"}, false
	}

	selected := make([]string, 0, len(allStatuses))
	seen := make(map[string]bool, len(allStatuses))
	for _, candidate := range raw {
		for _, part := range strings.Split(candidate, ",") {
			trimmed := strings.TrimSpace(part)
			if !slices.Contains(allStatuses, trimmed) || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			selected = append(selected, trimmed)
		}
	}

	if len(selected) == 0 {
		return []string{"Waiting", "Ready to buy"}, false
	}

	return selected, true
}

func filterAndSortItems(items []Item, searchQuery string, statuses []string, tagFilter string, sortBy string) []Item {
	trimmedSearch := strings.ToLower(strings.TrimSpace(searchQuery))
	trimmedTag := strings.ToLower(strings.TrimSpace(tagFilter))
	statusFilter := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		statusFilter[status] = true
	}
	hasStatusFilter := len(statusFilter) > 0 && len(statusFilter) < len(allStatuses)

	filtered := make([]Item, 0, len(items))
	for _, item := range items {
		if hasStatusFilter && !statusFilter[item.Status] {
			continue
		}

		if trimmedTag != "" && !itemHasTag(item.Tags, trimmedTag) {
			continue
		}

		if trimmedSearch != "" {
			haystack := strings.ToLower(strings.Join([]string{item.Title, item.Note, item.Link, item.Tags}, " "))
			if !strings.Contains(haystack, trimmedSearch) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	slices.SortStableFunc(filtered, func(a, b Item) int {
		switch sortBy {
		case "newest":
			if cmp := b.CreatedAt.Compare(a.CreatedAt); cmp != 0 {
				return cmp
			}
		case "oldest":
			if cmp := a.CreatedAt.Compare(b.CreatedAt); cmp != 0 {
				return cmp
			}
		case "price_asc", "price_desc":
			if a.HasPriceValue != b.HasPriceValue {
				if a.HasPriceValue {
					return -1
				}
				return 1
			}
			if a.HasPriceValue && b.HasPriceValue {
				if sortBy == "price_asc" {
					if cmp := a.PriceValue - b.PriceValue; cmp != 0 {
						if cmp < 0 {
							return -1
						}
						return 1
					}
				} else {
					if cmp := b.PriceValue - a.PriceValue; cmp != 0 {
						if cmp < 0 {
							return -1
						}
						return 1
					}
				}
			}
		default:
			statusRank := func(status string) int {
				switch status {
				case "Ready to buy":
					return 0
				case "Waiting":
					return 1
				default:
					return 2
				}
			}

			if cmp := statusRank(a.Status) - statusRank(b.Status); cmp != 0 {
				if cmp < 0 {
					return -1
				}
				return 1
			}

			if a.Status == "Ready to buy" || a.Status == "Waiting" {
				if cmp := a.PurchaseAllowedAt.Compare(b.PurchaseAllowedAt); cmp != 0 {
					return cmp
				}
			} else {
				if cmp := b.CreatedAt.Compare(a.CreatedAt); cmp != 0 {
					return cmp
				}
			}

			if cmp := b.CreatedAt.Compare(a.CreatedAt); cmp != 0 {
				return cmp
			}
			return b.ID - a.ID
		}

		if cmp := b.CreatedAt.Compare(a.CreatedAt); cmp != 0 {
			return cmp
		}
		return b.ID - a.ID
	})

	return filtered
}

func (a *App) renderHome(w http.ResponseWriter, r *http.Request, data homeViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	allItems := append([]Item(nil), a.items...)
	data.TotalItems = len(allItems)
	data.Currency = profileCurrencyOrDefault(a.currency)
	data.ActiveProfile = a.currentUserIDLocked()
	if parsedWage, err := parseHourlyWage(a.hourlyWage); err == nil {
		data.HourlyWage = parsedWage
		data.HasHourlyWage = true
	}
	data.SearchQuery = strings.TrimSpace(r.URL.Query().Get("q"))
	selectedStatuses, explicitStatusSelection := parseStatusFilter(r.URL.Query()["status"])
	data.SelectedStatus = make(map[string]bool, len(selectedStatuses))
	for _, status := range selectedStatuses {
		data.SelectedStatus[status] = true
	}
	data.TagFilter = strings.TrimSpace(r.URL.Query().Get("tag"))
	data.TagOptions = availableTagOptions(allItems)
	data.SortBy = normalizeSortBy(r.URL.Query().Get("sort"))
	data.HasActiveFilter = data.SearchQuery != "" || data.TagFilter != "" || data.SortBy != "next_ready" || explicitStatusSelection
	data.Items = filterAndSortItems(allItems, data.SearchQuery, selectedStatuses, data.TagFilter, data.SortBy)
	data.ContentTemplate = "index_content"
	data.ScriptTemplate = "index_script"
	a.mu.Unlock()

	renderTemplate(w, a.templates, "layout", data)
}

func (a *App) renderInsights(w http.ResponseWriter, data insightsViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.ItemCount = len(a.items)
	data.SkippedCount, data.SavedAmount, data.TopCategories = buildDashboardStats(a.items)
	data.DecisionTrend = buildMonthlyDecisionTrend(a.items)
	data.SavedTrend = buildMonthlySavedTrend(a.items)
	data.CategoryRatios = buildCategorySkipRatios(a.items)
	data.Currency = profileCurrencyOrDefault(a.currency)
	data.ActiveProfile = a.currentUserIDLocked()
	a.mu.Unlock()

	data.ContentTemplate = "insights_content"
	renderTemplate(w, a.templates, "layout", data)
}

func (a *App) renderItemForm(w http.ResponseWriter, data itemFormViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.Items = append([]Item(nil), a.items...)
	data.Currency = profileCurrencyOrDefault(a.currency)
	data.ActiveProfile = a.currentUserIDLocked()
	a.mu.Unlock()

	data.TagOptions = availableTagOptions(data.Items)
	data.SelectedTags = selectedTagsMap(data.FormValues.Tags)

	if data.FormValues.WaitPreset == "" {
		a.mu.RLock()
		data.FormValues.WaitPreset = defaultWaitPreset(a.defaultWaitPreset)
		if data.FormValues.WaitPreset == "custom" {
			data.FormValues.WaitCustomHours = a.defaultWaitCustomHours
		}
		a.mu.RUnlock()
	}

	if data.PurchaseAllowedInput == "" && !data.FormValues.PurchaseAllowedAt.IsZero() {
		data.PurchaseAllowedInput = data.FormValues.PurchaseAllowedAt.Format("2006-01-02T15:04")
	}

	if data.FormAction == "" {
		data.FormAction = "/items/new"
	}
	if data.SubmitLabel == "" {
		data.SubmitLabel = "Add to waitlist"
	}
	if data.CancelHref == "" {
		data.CancelHref = "/"
	}

	data.ContentTemplate = "items_new_content"
	data.ScriptTemplate = "items_new_script"
	renderTemplate(w, a.templates, "layout", data)
}

func (a *App) renderProfile(w http.ResponseWriter, data profileViewData) {
	a.mu.RLock()
	if data.ProfileName == "" {
		data.ProfileName = a.currentUserIDLocked()
	}
	if data.ProfileHourly == "" {
		data.ProfileHourly = a.hourlyWage
	}
	if data.NtfyEndpoint == "" {
		data.NtfyEndpoint = a.ntfyURL
	}
	if data.NtfyTopic == "" {
		data.NtfyTopic = a.ntfyTopic
	}
	if data.Currency == "" {
		data.Currency = profileCurrencyOrDefault(a.currency)
	}
	if data.ActiveProfile == "" {
		data.ActiveProfile = a.currentUserIDLocked()
	}
	if data.DefaultWaitPreset == "" {
		data.DefaultWaitPreset = defaultWaitPreset(a.defaultWaitPreset)
	}
	if data.DefaultWaitCustomHours == "" {
		data.DefaultWaitCustomHours = a.defaultWaitCustomHours
	}
	a.mu.RUnlock()

	data.ContentTemplate = "profile_content"
	data.ScriptTemplate = "profile_script"
	renderTemplate(w, a.templates, "layout", data)
}

func parseProfileName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("Please enter a profile name.")
	}
	if len([]rune(name)) > 64 {
		return "", errors.New("Profile name must be 64 characters or fewer.")
	}
	return name, nil
}

func (a *App) hasActiveProfile() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return strings.TrimSpace(a.activeUserID) != ""
}

func (a *App) currentUserIDLocked() string {
	if strings.TrimSpace(a.activeUserID) == "" {
		return defaultUserID
	}
	return a.activeUserID
}

func (a *App) activeProfileName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentUserIDLocked()
}

func (a *App) listProfileNames() ([]string, error) {
	a.mu.RLock()
	db := a.db
	a.mu.RUnlock()
	if db == nil {
		if a.activeProfileName() == defaultUserID {
			return nil, nil
		}
		return []string{a.activeProfileName()}, nil
	}

	rows, err := db.Query(`SELECT user_id FROM (
	SELECT user_id FROM profiles
	UNION
	SELECT user_id FROM items
) ORDER BY user_id COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list profile names: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan profile name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profile names: %w", err)
	}
	return names, nil
}

func (a *App) switchProfile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/switch-profile" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		names, err := a.listProfileNames()
		if err != nil {
			http.Error(w, "could not load profiles", http.StatusInternalServerError)
			return
		}
		renderTemplate(w, a.templates, "layout", profileSwitchViewData{Title: "Choose profile", CurrentPath: "/switch-profile", ContentTemplate: "switch_profile_content", Names: names, SelectedName: "", ActiveProfile: a.activeProfileName()})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		name, err := parseProfileName(r.FormValue("profile_name"))
		if err != nil {
			names, _ := a.listProfileNames()
			renderTemplate(w, a.templates, "layout", profileSwitchViewData{Title: "Choose profile", CurrentPath: "/switch-profile", ContentTemplate: "switch_profile_content", Names: names, SelectedName: "", Error: err.Error(), ActiveProfile: a.activeProfileName()})
			return
		}

		a.mu.Lock()
		a.activeUserID = name
		if err := a.loadStateFromDB(name); err != nil {
			a.mu.Unlock()
			http.Error(w, "could not switch profile", http.StatusInternalServerError)
			return
		}
		isNewProfile := !a.profileExists
		if strings.TrimSpace(a.hourlyWage) == "" {
			a.hourlyWage = defaultProfileHourlyWage
		}
		if strings.TrimSpace(a.currency) == "" {
			a.currency = normalizeCurrency("")
		}
		if err := a.persistProfileLocked(); err != nil {
			a.mu.Unlock()
			http.Error(w, "could not initialize profile", http.StatusInternalServerError)
			return
		}
		needsProfileSetup := isNewProfile
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "active_profile", Value: name, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		if needsProfileSetup {
			http.Redirect(w, r, "/settings/profile", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseHourlyWage(raw string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("Please enter a valid hourly wage (> 0).")
	}

	return parsed, nil
}

func parsePrice(raw string) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}

	return parsed, true
}

func (a *App) promoteReadyItemsLocked(now time.Time) {
	for i := range a.items {
		if a.items[i].Status != "Waiting" {
			continue
		}
		if !a.items[i].PurchaseAllowedAt.After(now) {
			a.items[i].Status = "Ready to buy"
			if err := a.updatePromotedItemLocked(a.items[i]); err != nil {
				log.Printf("db error while promoting item %d: %v", a.items[i].ID, err)
			}
			a.sendNtfyNotificationLocked(a.items[i])
		}
	}
}

func (a *App) sendNtfyNotificationLocked(item Item) {
	if item.NtfyAttempted {
		return
	}

	for i := range a.items {
		if a.items[i].ID == item.ID {
			a.items[i].NtfyAttempted = true
			if err := a.markNtfyAttemptedLocked(item.ID); err != nil {
				log.Printf("db error while marking ntfy attempt for item %d: %v", item.ID, err)
			}
			break
		}
	}

	if strings.TrimSpace(a.ntfyURL) == "" || strings.TrimSpace(a.ntfyTopic) == "" {
		log.Printf("ntfy skipped for item %d: endpoint/topic not configured", item.ID)
		return
	}

	message := fmt.Sprintf("%s is now ready to buy.\nDashboard: %s", item.Title, a.dashboardLink())
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", a.ntfyURL, a.ntfyTopic), strings.NewReader(message))
	if err != nil {
		log.Printf("ntfy request creation failed for item %d: %v", item.ID, err)
		return
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Title", "Impulse Pause reminder")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ntfy request failed for item %d: %v", item.ID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusInternalServerError {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("ntfy request returned %d for item %d: %s", resp.StatusCode, item.ID, strings.TrimSpace(string(body)))
	}
}

func (a *App) dashboardLink() string {
	if a.dashboardURL == "" {
		return "http://localhost:8080/"
	}
	return a.dashboardURL + "/"
}

func workHoursAvailable(item Item, hourlyWage float64, hasHourlyWage bool) bool {
	if !hasHourlyWage {
		return false
	}

	_, ok := parsePrice(item.Price)
	return ok
}

func formatWorkHours(item Item, hourlyWage float64) string {
	price, ok := parsePrice(item.Price)
	if !ok || hourlyWage <= 0 {
		return ""
	}

	hours := price / hourlyWage
	roundedHours := math.Round(hours*10) / 10
	return fmt.Sprintf("%.1f", roundedHours)
}

func normalizeCurrency(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "€"
	}
	return trimmed
}

func profileCurrencyOrDefault(raw string) string {
	return normalizeCurrency(raw)
}

func formatMoney(amount float64, currency string) string {
	return fmt.Sprintf("%s %.2f", profileCurrencyOrDefault(currency), amount)
}

func buildDashboardStats(items []Item) (skippedCount int, savedAmount float64, topCategories []categoryCount) {
	categoryTotals := map[string]int{}

	for _, item := range items {
		if item.Status == "Skipped" {
			skippedCount++
			if item.HasPriceValue {
				savedAmount += item.PriceValue
			}
		}

		for _, category := range categoriesFromTags(item.Tags) {
			categoryTotals[category]++
		}
	}

	for category, count := range categoryTotals {
		topCategories = append(topCategories, categoryCount{Name: category, Count: count})
	}

	slices.SortFunc(topCategories, func(a, b categoryCount) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Name, b.Name)
	})

	if len(topCategories) > 3 {
		topCategories = topCategories[:3]
	}

	return skippedCount, savedAmount, topCategories
}

func buildMonthlyDecisionTrend(items []Item) []monthlyDecisionTrend {
	monthly := map[string]*monthlyDecisionTrend{}
	for _, item := range items {
		if item.Status != "Bought" && item.Status != "Skipped" {
			continue
		}
		month := item.CreatedAt.Format("2006-01")
		bucket, exists := monthly[month]
		if !exists {
			bucket = &monthlyDecisionTrend{Month: month}
			monthly[month] = bucket
		}
		if item.Status == "Bought" {
			bucket.BoughtCount++
		} else {
			bucket.SkippedCount++
		}
	}

	if len(monthly) == 0 {
		return nil
	}

	months := mapKeys(monthly)
	slices.Sort(months)

	trends := make([]monthlyDecisionTrend, 0, len(months))
	for _, month := range months {
		trends = append(trends, *monthly[month])
	}

	return trends
}

func buildMonthlySavedTrend(items []Item) []monthlySavedAmount {
	monthly := map[string]float64{}
	for _, item := range items {
		if item.Status != "Skipped" || !item.HasPriceValue {
			continue
		}
		month := item.CreatedAt.Format("2006-01")
		monthly[month] += item.PriceValue
	}

	if len(monthly) == 0 {
		return nil
	}

	months := mapKeys(monthly)
	slices.Sort(months)

	trend := make([]monthlySavedAmount, 0, len(months))
	for _, month := range months {
		trend = append(trend, monthlySavedAmount{Month: month, Amount: monthly[month]})
	}

	return trend
}

func buildCategorySkipRatios(items []Item) []categorySkipRatio {
	decisions := map[string]int{}
	skips := map[string]int{}

	for _, item := range items {
		if item.Status != "Bought" && item.Status != "Skipped" {
			continue
		}

		for _, category := range categoriesFromTags(item.Tags) {
			decisions[category]++
			if item.Status == "Skipped" {
				skips[category]++
			}
		}
	}

	if len(decisions) == 0 {
		return nil
	}

	result := make([]categorySkipRatio, 0, len(decisions))
	for category, decisionCount := range decisions {
		skipped := skips[category]
		ratio := float64(skipped) / float64(decisionCount)
		result = append(result, categorySkipRatio{
			Name:          category,
			SkippedCount:  skipped,
			DecisionCount: decisionCount,
			Ratio:         ratio,
		})
	}

	slices.SortFunc(result, func(a, b categorySkipRatio) int {
		if a.Ratio != b.Ratio {
			if a.Ratio > b.Ratio {
				return -1
			}
			return 1
		}
		if a.DecisionCount != b.DecisionCount {
			return b.DecisionCount - a.DecisionCount
		}
		return strings.Compare(a.Name, b.Name)
	})

	if len(result) > 5 {
		result = result[:5]
	}

	return result
}

func mapKeys[T any](in map[string]T) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	return keys
}

func categoriesFromTags(rawTags string) []string {
	if strings.TrimSpace(rawTags) == "" {
		return nil
	}

	parts := strings.Split(rawTags, ",")
	seen := map[string]struct{}{}
	var categories []string

	for _, part := range parts {
		category := strings.ToLower(strings.TrimSpace(part))
		if category == "" {
			continue
		}
		if _, exists := seen[category]; exists {
			continue
		}
		seen[category] = struct{}{}
		categories = append(categories, category)
	}

	return categories
}

func availableTagOptions(items []Item) []string {
	options := make([]string, len(defaultTagOptions))
	copy(options, defaultTagOptions)

	seen := map[string]struct{}{}
	for _, option := range options {
		seen[strings.ToLower(option)] = struct{}{}
	}

	for _, item := range items {
		for _, category := range strings.Split(item.Tags, ",") {
			tag := strings.TrimSpace(category)
			if tag == "" {
				continue
			}
			key := strings.ToLower(tag)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			options = append(options, tag)
		}
	}

	slices.SortFunc(options, func(a string, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})

	return options
}

func selectedTagsMap(rawTags string) map[string]bool {
	selected := map[string]bool{}
	for _, part := range strings.Split(rawTags, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		selected[tag] = true
	}
	return selected
}

func parseTagsFromForm(selectedTags []string, customTag string) string {
	combined := append([]string{}, selectedTags...)
	if trimmedCustom := strings.TrimSpace(customTag); trimmedCustom != "" {
		combined = append(combined, trimmedCustom)
	}

	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(combined))
	for _, raw := range combined {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}

	return strings.Join(normalized, ", ")
}

func itemHasTag(rawTags string, normalizedTagFilter string) bool {
	for _, part := range strings.Split(rawTags, ",") {
		if strings.ToLower(strings.TrimSpace(part)) == normalizedTagFilter {
			return true
		}
	}
	return false
}

func mul100(v float64) float64 {
	return v * 100
}
func statusBadgeClass(status string) string {
	switch status {
	case "Ready to buy":
		return "text-bg-success"
	case "Bought":
		return "text-bg-primary"
	case "Skipped":
		return "text-bg-secondary"
	default:
		return "text-bg-warning"
	}
}

func (a *App) about(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, a.templates, "layout", pageData{Title: "About", CurrentPath: "/about", ContentTemplate: "about_content", ActiveProfile: a.activeProfileName()})
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func renderTemplate(w http.ResponseWriter, tpls *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpls.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
