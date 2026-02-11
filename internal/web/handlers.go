package web

import (
	"embed"
	"errors"
	"html/template"
	"log"
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
	Title      string
	Items      []Item
	FormValues Item
	Error      string
}

type pageData struct {
	Title string
}

type App struct {
	templates *template.Template
	mux       *http.ServeMux
	mu        sync.RWMutex
	items     []Item
	nextID    int
}

func NewApp() *App {
	tpls := template.Must(template.New("").Funcs(template.FuncMap{
		"statusBadgeClass": statusBadgeClass,
	}).ParseFS(embeddedFiles, "templates/*.html"))
	mux := http.NewServeMux()

	app := &App{templates: tpls, mux: mux, nextID: 1}
	app.routes()

	return app
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.home)
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
		a.renderHome(w, homeViewData{Title: "Impulse Pause"})
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

	if item.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		a.renderHome(w, homeViewData{
			Title:      "Impulse Pause",
			FormValues: item,
			Error:      "Bitte gib einen Titel ein.",
		})
		return
	}

	waitDuration, err := parseWaitDuration(item.WaitPreset, item.WaitCustomHours)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.renderHome(w, homeViewData{
			Title:      "Impulse Pause",
			FormValues: item,
			Error:      err.Error(),
		})
		return
	}

	item.Status = "Wartet"
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
	if !slices.Contains([]string{"Gekauft", "Nicht gekauft"}, newStatus) {
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

		if a.items[i].Status != "Kauf erlaubt" {
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
		hours, err := strconv.Atoi(strings.TrimSpace(waitCustomHours))
		if err != nil || hours <= 0 {
			return 0, errors.New("Bitte gib für Custom eine gültige Anzahl Stunden (> 0) ein.")
		}
		return time.Duration(hours) * time.Hour, nil
	default:
		return 0, errors.New("Bitte wähle eine gültige Wartezeit aus.")
	}
}

func (a *App) renderHome(w http.ResponseWriter, data homeViewData) {
	a.mu.Lock()
	a.promoteReadyItemsLocked(time.Now())
	data.Items = append([]Item(nil), a.items...)
	a.mu.Unlock()

	renderTemplate(w, a.templates, "index.html", data)
}

func (a *App) promoteReadyItemsLocked(now time.Time) {
	for i := range a.items {
		if a.items[i].Status != "Wartet" {
			continue
		}
		if !a.items[i].PurchaseAllowedAt.After(now) {
			a.items[i].Status = "Kauf erlaubt"
		}
	}
}

func statusBadgeClass(status string) string {
	switch status {
	case "Kauf erlaubt":
		return "text-bg-success"
	case "Gekauft":
		return "text-bg-primary"
	case "Nicht gekauft":
		return "text-bg-secondary"
	default:
		return "text-bg-warning"
	}
}

func (a *App) about(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, a.templates, "about.html", pageData{Title: "About"})
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
