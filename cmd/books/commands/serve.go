// Copyright © 2018 Author

package commands

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/pkg/errors"

	"github.com/tspivey/books"

	"strings"
	"sync"

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

var templates *template.Template
var cacheDir string

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringP("bind", "b", "127.0.0.1:8000", "Bind the server to host:port. Leave host empty to bind to all interfaces.")
	serveCmd.Flags().IntP("conversion-workers", "c", 4, "Number of conversion workers to run")
	viper.BindPFlag("server.bind", serveCmd.Flags().Lookup("bind"))
	viper.BindPFlag("server.conversion_workers", serveCmd.Flags().Lookup("conversion-workers"))
}

type libHandler struct {
	lib           *books.Library
	convertingMtx sync.Mutex
	converting    map[int64]error // Holds book conversion status
	fileCh        chan *books.BookFile
}

func runServer(cmd *cobra.Command, args []string) {
	cacheDir = path.Join(cfgDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %s\n", err)
		os.Exit(1)
	}
	templatesDir := path.Join(cfgDir, "templates")
	funcMap := template.FuncMap{"joinNaturally": joinNaturally}
	templates = template.Must(template.New("template").Funcs(funcMap).ParseGlob(path.Join(templatesDir, "*.html")))
	lib, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening library: %s\n", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	lh := libHandler{
		lib:        lib,
		fileCh:     make(chan *books.BookFile),
		converting: make(map[int64]error),
	}

	numConversionWorkers := viper.GetInt("server.conversion_workers")
	for i := 0; i < numConversionWorkers; i++ {
		go bookConverterWorker(&lh)
	}
	log.Printf("Started %d workers for converting books", numConversionWorkers)

	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/book/{id:\\d+}", lh.bookDetailsHandler)
	r.HandleFunc("/download/{id:\\d+}", lh.downloadHandler)
	r.HandleFunc("/search/", lh.searchHandler)

	secProvider := auth.HtpasswdFileProvider(htpasswdFile)
	authHandler := auth.NewBasicAuthenticator("Basic Realm", secProvider)
	handler := http.Handler(r)
	if _, err := os.Stat(htpasswdFile); err == nil {
		handler = auth.JustCheck(authHandler, handler.ServeHTTP)
		log.Printf("Using htpasswd file: %s\n", htpasswdFile)
	}

	srv := &http.Server{
		Addr:         viper.GetString("server.bind"),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      handler,
	}

	log.Printf("Listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	render("index", w, nil)
}

func (h *libHandler) downloadHandler(w http.ResponseWriter, r *http.Request) {
	file_id := mux.Vars(r)["id"]
	id, err := strconv.Atoi(file_id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	files, err := h.lib.GetFilesById([]int64{int64(id)})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(files) == 0 {
		render("error_page", w, errorPage{"File not found", "That file doesn't exist in the library."})
		return
	}
	file := files[0]

	fn := path.Join(booksRoot, file.CurrentFilename)
	base := path.Base(fn)
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Printf("File %d is in the library but the file is missing: %s", file.Id, fn)
		render("error_page", w, errorPage{"Cannot download file", "It looks like that file is in the library, but the file is missing."})
		return
	}

	var epubFn string
	if val, ok := r.URL.Query()["format"]; ok && val[0] == "epub" {
		epubFn = path.Join(cacheDir, file.Hash+".epub")
		if _, err := os.Stat(epubFn); os.IsNotExist(err) {
			h.convertingMtx.Lock()
			err, converting := h.converting[file.Id]
			h.convertingMtx.Unlock()
			if !converting {
				select {
				case h.fileCh <- &file:
					w.Header().Set("Refresh", "15")
					render("converting", w, file)
				default:
					render("error_page", w, errorPage{"Conversion error", "The conversion queue is full. Try again later."})
				}
				return
			}
			if err != nil {
				render("error_page", w, errorPage{"Conversion error", "That file couldn't be converted."})
				log.Printf("File %d couldn't be converted: %s", file.Id, err)
				h.convertingMtx.Lock()
				delete(h.converting, file.Id)
				h.convertingMtx.Unlock()
				return
			}
			w.Header().Set("Refresh", "15")
			render("converting", w, file)
			return
		}

		n := strings.TrimSuffix(base, path.Ext(base)) + ".epub"
		w.Header().Set("Content-Disposition", "attachment; filename=\""+n+"\"")
		http.ServeFile(w, r, epubFn)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+base+"\"")
	http.ServeFile(w, r, fn)
}

func (h *libHandler) bookDetailsHandler(w http.ResponseWriter, r *http.Request) {
	bookId := mux.Vars(r)["id"]
	id, err := strconv.Atoi(bookId)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	books, err := h.lib.GetBooksById([]int64{int64(id)})
	if err != nil {
		log.Printf("Error getting books by ID: %s", err)
		http.NotFound(w, r)
		return
	}
	if len(books) == 0 {
		render("error_page", w, errorPage{"Book not found", "That book doesn't exist in the library."})
		return
	}
	book := books[0]

	render("book_details", w, book)
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

func (h *libHandler) searchHandler(w http.ResponseWriter, r *http.Request) {
	pageNumber, offset, limit := 1, 0, 20
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

	books, moreResults, err := h.lib.SearchPaged(val[0], offset, limit, limit*(maxPageLinks-1))
	if err != nil {
		log.Printf("Error searching for %s: %s", val[0], err)
		render("error_page", w, errorPage{"Error while searching", "An error occurred while searching."})
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
	render("results", w, res)
}

// render renders the template specified by name to w, and sets dot (.) to data.
func render(name string, w http.ResponseWriter, data interface{}) {
	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Println(errors.Wrap(err, "rendering template"))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")
		return
	}
}

// bookConverterWorker listens on h.booksCh for books to convert to epub.
func bookConverterWorker(h *libHandler) {
	for book := range h.fileCh {
		// ok is true for _, ok := map[key] even for nil values.
		// Add a nil error to signal that a conversion is taking place.
		h.convertingMtx.Lock()
		h.converting[book.Id] = nil
		h.convertingMtx.Unlock()

		err := h.lib.ConvertToEpub(*book)
		h.convertingMtx.Lock()
		if err != nil {
			h.converting[book.Id] = err
		} else {
			delete(h.converting, book.Id)
		}
		h.convertingMtx.Unlock()
	}
}
