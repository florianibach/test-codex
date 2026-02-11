package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

type Item struct {
	Title     string
	Price     string
	Link      string
	Note      string
	Tags      string
	Status    string
	CreatedAt time.Time
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
}

func NewApp() *App {
	tpls := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	mux := http.NewServeMux()

	app := &App{templates: tpls, mux: mux}
	app.routes()

	return app
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.home)
	a.mux.HandleFunc("/healthz", a.health)
	a.mux.HandleFunc("/about", a.about)
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
	case http.MethodGet:
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
		Title: strings.TrimSpace(r.FormValue("title")),
		Price: strings.TrimSpace(r.FormValue("price")),
		Link:  strings.TrimSpace(r.FormValue("link")),
		Note:  strings.TrimSpace(r.FormValue("note")),
		Tags:  strings.TrimSpace(r.FormValue("tags")),
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

	item.Status = "Wartet"
	item.CreatedAt = time.Now()

	a.mu.Lock()
	a.items = append([]Item{item}, a.items...)
	a.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) renderHome(w http.ResponseWriter, data homeViewData) {
	a.mu.RLock()
	data.Items = append([]Item(nil), a.items...)
	a.mu.RUnlock()

	renderTemplate(w, a.templates, "index.html", data)
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
