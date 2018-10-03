// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

// Copyright © 2018 Author

package commands

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/pkg/errors"

	"github.com/tspivey/books"
	"github.com/tspivey/books/server"

	"strings"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the library from a web server",
	Long:  `Bring up a web server and serve the library.`,
	Run:   runServer,
}

var cacheDir string
var itemsPerPage int

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringP("bind", "b", "127.0.0.1:8000", "Bind the server to host:port. Leave host empty to bind to all interfaces.")
	serveCmd.Flags().IntP("conversion-workers", "c", 4, "Number of conversion workers to run")
	viper.BindPFlag("server.bind", serveCmd.Flags().Lookup("bind"))
	viper.BindPFlag("server.conversion_workers", serveCmd.Flags().Lookup("conversion-workers"))
	viper.SetDefault("server.read_timeout", 5)
	viper.SetDefault("server.write_timeout", 0)
	viper.SetDefault("server.idle_timeout", 120)
	viper.SetDefault("server.items_per_page", 20)
}

type Server struct {
	lib       *books.Library
	converter server.BookConverter
	templates *template.Template
}

func runServer(cmd *cobra.Command, args []string) {
	cacheDir = path.Join(cfgDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %s\n", err)
		os.Exit(1)
	}
	templatesDir := path.Join(cfgDir, "templates")
	lib, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening library: %s\n", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	numConversionWorkers := viper.GetInt("server.conversion_workers")
	converter := server.NewCalibreBookConverter(booksRoot, cacheDir, numConversionWorkers)
	srv := New(lib, templatesDir, converter)
	log.Printf("Started %d workers for converting books", numConversionWorkers)

	itemsPerPage = viper.GetInt("server.items_per_page")

	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc("/book/{id:\\d+}", srv.bookDetailsHandler)
	r.HandleFunc("/download/{id:\\d+}/{name:.+}", srv.downloadHandler)
	r.HandleFunc("/download/{id:\\d+}", srv.downloadHandler)
	r.HandleFunc("/search/", srv.searchHandler)

	secProvider := auth.HtpasswdFileProvider(htpasswdFile)
	authHandler := auth.NewBasicAuthenticator("Basic Realm", secProvider)
	handler := http.Handler(r)
	if _, err := os.Stat(htpasswdFile); err == nil {
		handler = auth.JustCheck(authHandler, handler.ServeHTTP)
		log.Printf("Using htpasswd file: %s\n", htpasswdFile)
	}

	hsrv := &http.Server{
		Addr:         viper.GetString("server.bind"),
		ReadTimeout:  viper.GetDuration("server.read_timeout") * time.Second,
		WriteTimeout: viper.GetDuration("server.write_timeout") * time.Second,
		IdleTimeout:  viper.GetDuration("server.idle_timeout") * time.Second,
		Handler:      handler,
	}

	log.Printf("Listening on %s", hsrv.Addr)
	log.Printf("Read timeout: %d, write timeout: %d, idle timeout: %d seconds", hsrv.ReadTimeout/time.Second, hsrv.WriteTimeout/time.Second, hsrv.IdleTimeout/time.Second)
	log.Fatal(hsrv.ListenAndServe())
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	srv.render("index", w, nil)
}

func (srv *Server) downloadHandler(w http.ResponseWriter, r *http.Request) {
	fileID := mux.Vars(r)["id"]
	id, err := strconv.Atoi(fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	files, err := srv.lib.GetFilesByID([]int64{int64(id)})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(files) == 0 {
		srv.render("error_page", w, errorPage{"File not found", "That file doesn't exist in the library."})
		return
	}
	file := files[0]

	fn := path.Join(booksRoot, file.CurrentFilename)
	base := path.Base(fn)
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Printf("File %d is in the library but the file is missing: %s", file.ID, fn)
		srv.render("error_page", w, errorPage{"Cannot download file", "It looks like that file is in the library, but the file is missing."})
		return
	}

	if val, ok := r.URL.Query()["format"]; ok && val[0] == "epub" {
		epubFn, err := srv.converter.Convert(file)
		if err == server.ErrBookNotReady {
			w.Header().Set("Refresh", "15")
			srv.render("converting", w, file)
			return
		}
		if err == server.ErrQueueFull {
			srv.render("error_page", w, errorPage{"Conversion error", "The conversion queue is full. Try again later."})
			return
		}
		if err != nil {
			srv.render("error_page", w, errorPage{"Conversion error", "That file couldn't be converted."})
			return
		}

		n := strings.TrimSuffix(base, path.Ext(base)) + ".epub"
		if _, nameFound := mux.Vars(r)["name"]; !nameFound {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+n+"\"")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, epubFn)
		return
	}

	if _, nameFound := mux.Vars(r)["name"]; !nameFound {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+base+"\"")
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fn)
}

func (srv *Server) bookDetailsHandler(w http.ResponseWriter, r *http.Request) {
	bookID := mux.Vars(r)["id"]
	id, err := strconv.Atoi(bookID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	books, err := srv.lib.GetBooksByID([]int64{int64(id)})
	if err != nil {
		log.Printf("Error getting books by ID: %s", err)
		http.NotFound(w, r)
		return
	}
	if len(books) == 0 {
		srv.render("error_page", w, errorPage{"Book not found", "That book doesn't exist in the library."})
		return
	}
	book := books[0]

	srv.render("book_details", w, book)
}

type results struct {
	Books      []books.Book
	PageNumber int
	Prev       int
	Next       int
	PageLinks  []int
	Query      string
}

type errorPage struct {
	Short string
	Long  string
}

func (srv *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	pageNumber, offset, limit := 1, 0, itemsPerPage
	maxPageLinks := 10
	val, ok := r.URL.Query()["query"]
	if !ok {
		http.Redirect(w, r, "/", 301)
		return
	}
	if pageStrs, ok := r.URL.Query()["page"]; ok {
		if page, err := strconv.Atoi(pageStrs[0]); err == nil && page >= 1 {
			pageNumber = page
			offset = (pageNumber - 1) * limit // Pages start from 1, offsets from 0.
		}
	}

	books, moreResults, err := srv.lib.SearchPaged(val[0], offset, limit, limit*(maxPageLinks-1))
	if err != nil {
		log.Printf("Error searching for %s: %s", val[0], err)
		srv.render("error_page", w, errorPage{"Error while searching", "An error occurred while searching."})
		return
	}

	morePages := int(math.Ceil(float64(moreResults) / float64(limit)))
	firstPageLink := pageNumber - int(math.Ceil(float64(maxPageLinks)/2)) + 1
	if firstPageLink < 1 {
		firstPageLink = 1
	}
	lastPageLink := maxPageLinks/2 + pageNumber
	if lastPageLink < maxPageLinks {
		lastPageLink = maxPageLinks
	}
	if lastPageLink > pageNumber+morePages {
		lastPageLink = pageNumber + morePages
	}
	pageLinks := make([]int, 0)
	for i := firstPageLink; i <= lastPageLink; i++ {
		pageLinks = append(pageLinks, i)
	}
	nextPage := 0
	if morePages > 0 {
		nextPage = pageNumber + 1
	}

	res := results{
		Books:      books,
		PageNumber: pageNumber,
		Prev:       pageNumber - 1,
		Next:       nextPage,
		PageLinks:  pageLinks,
		Query:      val[0],
	}
	srv.render("results", w, res)
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

// changeExt changes the extension of pathname to ext, which should include ..
func changeExt(pathname string, ext string) string {
	return strings.TrimSuffix(pathname, path.Ext(pathname)) + ext
}

func New(lib *books.Library, templatesDir string, converter server.BookConverter) *Server {
	htmlFuncMap := template.FuncMap{
		"joinNaturally": joinNaturally,
		"noEscapeHTML":  func(s string) template.HTML { return template.HTML(s) },
		"searchFor":     searchFor,
		"base":          path.Base,
		"pathEscape":    url.PathEscape,
		"changeExt":     changeExt,
	}
	return &Server{
		lib:       lib,
		templates: template.Must(template.New("template").Funcs(htmlFuncMap).ParseGlob(path.Join(templatesDir, "*.html"))),
		converter: converter,
	}
}
