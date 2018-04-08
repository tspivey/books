// Copyright © 2018 Author

package commands

import (
	"fmt"
	"github.com/pkg/errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/tspivey/books"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
	"sync"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the library from a web server",
	Long:  `Bring up a web server and serve the library.`,
	Run:   runServer,
}

var templates *template.Template

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	serveCmd.Flags().StringP("bind", "b", "127.0.0.1:8000", "Bind the server to host:port. Leave host empty to bind to all interfaces.")
	serveCmd.Flags().IntP("conversion-workers", "c", 4, "Number of conversion workers to run")
	viper.BindPFlag("server.bind", serveCmd.Flags().Lookup("bind"))
	viper.BindPFlag("server.conversion_workers", serveCmd.Flags().Lookup("conversion-workers"))
}

type libHandler struct {
	lib           *books.Library
	convertingMtx sync.Mutex
	// Map holding book conversion status.
	converting map[int64]error
	bookCh     chan *books.Book
}

var cacheDir string

func runServer(cmd *cobra.Command, args []string) {
	templates = template.Must(template.ParseGlob("templates/*.html"))
	r := mux.NewRouter()
	lib, err := books.OpenLibrary(libraryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening library: %s\n", err)
		os.Exit(1)
	}
	cacheDir = path.Join(path.Dir(libraryFile), "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatal("Error creating cache directory: ", err)
	}
	lh := libHandler{lib: lib, bookCh: make(chan *books.Book), converting: make(map[int64]error)}
	for i := 0; i < viper.GetInt("server.conversion_workers"); i++ {
		go bookConverterWorker(&lh)
	}
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/download/{id:\\d+}", lh.downloadHandler)
	r.HandleFunc("/search/", lh.searchHandler)
	http.Handle("/", r)

	bindAddr := viper.GetString("server.bind")
	log.Printf("Listening on %s", bindAddr)
	log.Fatal(http.ListenAndServe(bindAddr, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	render("index", w, nil)
}

func (h *libHandler) downloadHandler(w http.ResponseWriter, r *http.Request) {
	book_id := mux.Vars(r)["id"]
	id, err := strconv.Atoi(book_id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	books, err := h.lib.GetBooksById([]int64{int64(id)})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(books) == 0 {
		render("error_page", w, errorPage{"Book not found", "That book doesn't exist in the library."})
		return
	}
	book := books[0]
	root := viper.GetString("root")
	fn := path.Join(root, book.CurrentFilename)
	base := path.Base(fn)
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Printf("Book %d is in the library but the file is missing: %s", book.Id, fn)
		render("error_page", w, errorPage{"Cannot download book", "It looks like that book is in the library, but the file is missing."})
		return
	}
	var epubFn string
	if val, ok := r.URL.Query()["format"]; ok && val[0] == "epub" {
		epubFn = path.Join(cacheDir, book.Hash+".epub")
		if _, err := os.Stat(epubFn); os.IsNotExist(err) {
			h.convertingMtx.Lock()
			err, converting := h.converting[book.Id]
			h.convertingMtx.Unlock()
			if !converting {
				select {
				case h.bookCh <- &book:
					render("converting", w, nil)
				default:
					render("error_page", w, errorPage{"Conversion error", "The conversion queue is full. Try again later."})
				}
				return
			} else if err != nil {
				render("error_page", w, errorPage{"Conversion error", "That book couldn't be converted."})
				log.Printf("Book %d couldn't be converted: %s", book.Id, err)
				h.convertingMtx.Lock()
				delete(h.converting, book.Id)
				h.convertingMtx.Unlock()
				return
			} else {
				render("converting", w, nil)
			}
		} else {
			n := strings.TrimSuffix(base, path.Ext(base)) + ".epub"
			w.Header().Set("Content-Disposition", "attachment; filename=\""+n+"\"")
			http.ServeFile(w, r, epubFn)
			return
		}
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+base+"\"")
	http.ServeFile(w, r, fn)
}

type results struct {
	Books []books.Book
	Query string
}

type errorPage struct {
	Short string
	Long  string
}

func (h *libHandler) searchHandler(w http.ResponseWriter, r *http.Request) {
	val, ok := r.URL.Query()["query"]
	if !ok {
		http.Redirect(w, r, "/", 301)
		return
	}
	books, err := h.lib.Search(val[0])
	if err != nil {
		log.Printf("Error searching for %s: %s", val[0], err)
	}
	res := results{Books: books, Query: val[0]}
	render("results", w, res)
}

func render(name string, w http.ResponseWriter, data interface{}) {
	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Println(errors.Wrap(err, "rendering template"))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error")
		return
	}
}

func bookConverterWorker(h *libHandler) {
	for book := range h.bookCh {
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
