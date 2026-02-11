package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
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

type itemFormViewData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	Items           []Item
	FormValues      Item
	Error           string
}

type profileViewData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
	ProfileHourly   string
	ProfileError    string
	ProfileFeedback string
}

type pageData struct {
	Title           string
	CurrentPath     string
	ContentTemplate string
	ScriptTemplate  string
}

type App struct {
	templates  *template.Template
	mux        *http.ServeMux
	mu         sync.RWMutex
	items      []Item
	hourlyWage string
	nextID     int
}

func NewApp() *App {
	tpls := template.Must(template.New("").Funcs(template.FuncMap{
		"statusBadgeClass":   statusBadgeClass,
		"workHoursAvailable": workHoursAvailable,
		"formatWorkHours":    formatWorkHours,
	}).ParseFS(embeddedFiles, "templates/*.html"))
	mux := http.NewServeMux()

	app := &App{templates: tpls, mux: mux, nextID: 1}
	app.routes()

	return app
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.home)
	a.mux.HandleFunc("/items/new", a.itemForm)
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

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		a.renderHome(w, homeViewData{Title: "Impulse Pause", CurrentPath: "/"})
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

	waitDuration, err := parseWaitDuration(item.WaitPreset, item.WaitCustomHours)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderItemForm(w, itemFormViewData{
			Title:       "Add item",
			CurrentPath: "/items/new",
			FormValues:  item,
			Error:       err.Error(),
		})
		return
	}

	item.Status = "Waiting"
	if item.WaitPreset == "" {
		item.WaitPreset = "24h"
	}
	item.CreatedAt = time.Now()
	item.PurchaseAllowedAt = item.CreatedAt.Add(waitDuration)

	a.mu.Lock()
	item.ID = a.nextID
	a.nextID++
	a.items = append([]Item{item}, a.items...)
	a.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
	if _, err := parseHourlyWage(hourlyWage); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderProfile(w, profileViewData{
			Title:         "Profile settings",
			CurrentPath:   "/settings/profile",
			ProfileHourly: hourlyWage,
			ProfileError:  err.Error(),
		})
		return
	}

	a.mu.Lock()
	a.hourlyWage = hourlyWage
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func parseWaitDuration(waitPreset string, waitCustomHours string) (time.Duration, error) {
	if waitPreset == "" {
		waitPreset = "24h"
	}

	switch waitPreset {
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

func (a *App) renderItemForm(w http.ResponseWriter, data itemFormViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.Items = append([]Item(nil), a.items...)
	a.mu.Unlock()

	data.ContentTemplate = "items_new_content"
	data.ScriptTemplate = "items_new_script"
	renderTemplate(w, a.templates, "layout", data)
}

func (a *App) renderProfile(w http.ResponseWriter, data profileViewData) {
	a.mu.RLock()
	if data.ProfileHourly == "" {
		data.ProfileHourly = a.hourlyWage
	}
	a.mu.RUnlock()

	data.ContentTemplate = "profile_content"
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
		}
	}
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
