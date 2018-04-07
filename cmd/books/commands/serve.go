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

var mapMutex sync.Mutex
var runningConversions map[int64]bool

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
	viper.BindPFlag("server.bind", serveCmd.Flags().Lookup("bind"))
}

type libHandler struct {
	lib *books.Library
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
	lh := libHandler{lib: lib}
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
			// Only start one conversion process per book
			mapMutex.Lock()
			if _, ok := runningConversions[book.Id]; !ok {
				go func() {
					log.Printf("Converting book %d to epub", book.Id)
					err := h.lib.ConvertToEpub(book)
					if err != nil {
						log.Printf("Error converting book %d to epub: %s", book.Id, err)
					}
					log.Printf("Converted book %d to epub", book.Id)
					mapMutex.Lock()
					delete(runningConversions, book.Id)
					mapMutex.Unlock()
				}()
				mapMutex.Unlock()
				render("converting", w, nil)
				return
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
