package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

type App struct {
	templates *template.Template
	mux       *http.ServeMux
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

	renderTemplate(w, a.templates, "index.html", map[string]string{"Title": "MVP Home"})
}

func (a *App) about(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, a.templates, "about.html", map[string]string{"Title": "About"})
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func renderTemplate(w http.ResponseWriter, tpls *template.Template, name string, data map[string]string) {
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
