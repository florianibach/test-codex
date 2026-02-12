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
	HourlyWage      float64
	HasHourlyWage   bool
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
}

type categoryCount struct {
	Name  string
	Count int
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
	PurchaseAllowedInput string
	Error                string
}

type profileViewData struct {
	Title                  string
	CurrentPath            string
	ContentTemplate        string
	ScriptTemplate         string
	ProfileHourly          string
	DefaultWaitPreset      string
	DefaultWaitCustomHours string
	NtfyEndpoint           string
	NtfyTopic              string
	ProfileError           string
	ProfileFeedback        string
}

type pageData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
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
	dashboardURL           string
	nextID                 int
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
	}).ParseFS(embeddedFiles, "templates/*.html"))
	mux := http.NewServeMux()

	app := &App{templates: tpls, mux: mux, db: db, nextID: 1}
	if err := app.loadStateFromDB(); err != nil {
		return nil, err
	}
	app.routes()
	app.StartBackgroundPromotion(5 * time.Second)

	return app, nil
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.home)
	a.mux.HandleFunc("/items/new", a.itemForm)
	a.mux.HandleFunc("/items/edit", a.editItemForm)
	a.mux.HandleFunc("/insights", a.insights)
	a.mux.HandleFunc("/settings/profile", a.profileSettings)
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

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if !a.hasProfile() {
			http.Redirect(w, r, "/settings/profile", http.StatusSeeOther)
			return
		}
		a.renderHome(w, homeViewData{Title: "Impulse Pause", CurrentPath: "/"})
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
		Tags:            strings.TrimSpace(r.FormValue("tags")),
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
	purchaseAllowedAt, err := resolvePurchaseAllowedAt(item.WaitPreset, item.WaitCustomHours, purchaseAllowedInput, now)
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
		Tags:            strings.TrimSpace(r.FormValue("tags")),
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
	purchaseAllowedAt, err := resolvePurchaseAllowedAt(item.WaitPreset, item.WaitCustomHours, purchaseAllowedInput, now)
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

	hourlyWage := strings.TrimSpace(r.FormValue("hourly_wage"))
	defaultPreset := strings.TrimSpace(r.FormValue("default_wait_preset"))
	defaultCustomHours := strings.TrimSpace(r.FormValue("default_wait_custom_hours"))
	ntfyURL := strings.TrimRight(strings.TrimSpace(r.FormValue("ntfy_endpoint")), "/")
	ntfyTopic := strings.TrimSpace(r.FormValue("ntfy_topic"))

	if _, err := parseHourlyWage(hourlyWage); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			ProfileError:           err.Error(),
		})
		return
	}

	if _, err := parseWaitDuration(defaultPreset, defaultCustomHours); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			ProfileError:           err.Error(),
		})
		return
	}

	if (ntfyURL == "" && ntfyTopic != "") || (ntfyURL != "" && ntfyTopic == "") {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:                  "Profile settings",
			CurrentPath:            "/settings/profile",
			ProfileHourly:          hourlyWage,
			DefaultWaitPreset:      defaultPreset,
			DefaultWaitCustomHours: defaultCustomHours,
			NtfyEndpoint:           ntfyURL,
			NtfyTopic:              ntfyTopic,
			ProfileError:           "Please provide both ntfy endpoint and topic, or leave both empty.",
		})
		return
	}

	a.mu.Lock()
	a.hourlyWage = hourlyWage
	a.defaultWaitPreset = defaultWaitPreset(defaultPreset)
	if a.defaultWaitPreset == "custom" {
		a.defaultWaitCustomHours = defaultCustomHours
	} else {
		a.defaultWaitCustomHours = ""
	}
	a.ntfyURL = ntfyURL
	a.ntfyTopic = ntfyTopic
	if err := a.persistProfileLocked(); err != nil {
		a.mu.Unlock()
		log.Printf("db error while saving profile: %v", err)
		http.Error(w, "could not save profile", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()

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

func parsePurchaseAllowedAt(raw string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02T15:04", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, errors.New("Please enter a valid buy-after date and time.")
	}
	return parsed, nil
}

func resolvePurchaseAllowedAt(waitPreset string, waitCustomHours string, purchaseAllowedRaw string, now time.Time) (time.Time, error) {
	if normalizeItemWaitPreset(waitPreset) == "date" {
		if strings.TrimSpace(purchaseAllowedRaw) == "" {
			return time.Time{}, errors.New("Please enter a buy-after date and time.")
		}
		return parsePurchaseAllowedAt(purchaseAllowedRaw)
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
	return strings.TrimSpace(a.hourlyWage) != ""
}

func (a *App) renderHome(w http.ResponseWriter, data homeViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.Items = append([]Item(nil), a.items...)
	if parsedWage, err := parseHourlyWage(a.hourlyWage); err == nil {
		data.HourlyWage = parsedWage
		data.HasHourlyWage = true
	}
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
	a.mu.Unlock()

	data.ContentTemplate = "insights_content"
	renderTemplate(w, a.templates, "layout", data)
}

func (a *App) renderItemForm(w http.ResponseWriter, data itemFormViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.Items = append([]Item(nil), a.items...)
	a.mu.Unlock()

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
	if data.ProfileHourly == "" {
		data.ProfileHourly = a.hourlyWage
	}
	if data.NtfyEndpoint == "" {
		data.NtfyEndpoint = a.ntfyURL
	}
	if data.NtfyTopic == "" {
		data.NtfyTopic = a.ntfyTopic
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
	renderTemplate(w, a.templates, "layout", pageData{Title: "About", CurrentPath: "/about", ContentTemplate: "about_content"})
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
