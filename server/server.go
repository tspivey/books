package server

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	txtTemplate "text/template"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/tspivey/books"
)

// Server is the web server which handles searching for, downloading and converting books.
type Server struct {
	lib            *books.Library
	converter      BookConverter
	templates      *template.Template
	hsrv           *http.Server
	itemsPerPage   int
	booksRoot      string
	outputTemplate *txtTemplate.Template
}

// Config is the configuration of the server, used in New.
type Config struct {
	Lib            *books.Library
	TemplatesDir   string
	Converter      BookConverter
	ItemsPerPage   int
	Hsrv           *http.Server
	HtpasswdFile   string
	BooksRoot      string
	OutputTemplate *txtTemplate.Template
}

// New creates a new server.
func New(cfg *Config) *Server {
	htmlFuncMap := template.FuncMap{
		"joinNaturally": books.JoinNaturally,
		"noEscapeHTML":  func(s string) template.HTML { return template.HTML(s) },
		"searchFor":     searchFor,
		"base":          path.Base,
		"pathEscape":    url.PathEscape,
		"changeExt":     changeExt,
		"ByteCountSI":   books.ByteCountSI,
	}
	srv := &Server{
		lib:            cfg.Lib,
		templates:      template.Must(template.New("template").Funcs(htmlFuncMap).ParseGlob(path.Join(cfg.TemplatesDir, "*.html"))),
		converter:      cfg.Converter,
		hsrv:           cfg.Hsrv,
		itemsPerPage:   cfg.ItemsPerPage,
		booksRoot:      cfg.BooksRoot,
		outputTemplate: cfg.OutputTemplate,
	}

	r := mux.NewRouter()
	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc("/book/{id:\\d+}", srv.bookDetailsHandler)
	r.HandleFunc("/download/{id:\\d+}/{name:.+}", srv.downloadHandler)
	r.HandleFunc("/download/{id:\\d+}", srv.downloadHandler)
	r.HandleFunc("/search/", srv.searchHandler)
	apiRouter := r.PathPrefix("/api/").Subrouter()
	key := os.Getenv("BOOKS_API_KEY")
	if key == "" {
		log.Printf("Warning: BOOKS_API_KEY not set; API disabled")
	}
	apiLock := &sync.Mutex{}
	apiRouter.Use(func(next http.Handler) http.Handler {
		return apiKeyMiddleware(key, next, apiLock)
	})
	apiRouter.HandleFunc(`/book/{id:\d+}`, srv.getBookHandler)
	apiRouter.HandleFunc("/update", srv.updateBookHandler).Methods("POST")
	apiRouter.HandleFunc("/merge", srv.mergeHandler).Methods("POST")
	apiRouter.HandleFunc("/search", srv.apiSearchHandler)
	secProvider := auth.HtpasswdFileProvider(cfg.HtpasswdFile)
	authHandler := auth.NewBasicAuthenticator("Basic Realm", secProvider)
	handler := http.Handler(r)
	if _, err := os.Stat(cfg.HtpasswdFile); err == nil {
		handler = auth.JustCheck(authHandler, handler.ServeHTTP)
		log.Printf("Using htpasswd file: %s\n", cfg.HtpasswdFile)
	}
	srv.hsrv.Handler = handler
	return srv
}

// Start starts the server.
func (srv *Server) Start() error {
	return srv.hsrv.ListenAndServe()
}

// render renders the template specified by name to w, and sets dot (.) to data.
func (srv *Server) render(name string, w http.ResponseWriter, data interface{}) {
	err := srv.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Println(errors.Wrap(err, "rendering template"))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")
		return
	}
}

// searchFor wraps each item in a slice of strings with
// a link to search for that item in the library.
// Spaces will be replaced with +.
// If field is not empty, the search will be limited to that field.
func searchFor(field string, items []string) []string {
	if field != "" {
		field += ":"
	}

	newItems := make([]string, len(items))
	for i := range items {
		newItems[i] = fmt.Sprintf(`<a href="/search/?query=%s%s">%s</a>`,
			field, html.EscapeString(strings.Replace(items[i], " ", "%2B", -1)),
			html.EscapeString(items[i]))
	}

	return newItems
}

// changeExt changes the extension of pathname to ext. ext must include a preceding dot.
func changeExt(pathname string, ext string) string {
	return strings.TrimSuffix(pathname, path.Ext(pathname)) + ext
}

func apiKeyMiddleware(key string, next http.Handler, apiLock *sync.Mutex) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key == "" || r.Header.Get("x-API-key") != key {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, apiError{"forbidden"})
			return
		}
		apiLock.Lock()
		next.ServeHTTP(w, r)
		apiLock.Unlock()
	})
}
