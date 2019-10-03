package server

import (
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/tspivey/books"
)

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

	fn := path.Join(srv.booksRoot, file.HashPath())
	base := path.Base(fn)
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Printf("File %d is in the library but the file is missing: %s", file.ID, fn)
		srv.render("error_page", w, errorPage{"Cannot download file", "It looks like that file is in the library, but the file is missing."})
		return
	}

	if val, ok := r.URL.Query()["format"]; ok && val[0] == "epub" {
		epubFn, err := srv.converter.Convert(file)
		if err == errBookNotReady {
			w.Header().Set("Refresh", "15")
			srv.render("converting", w, file)
			return
		}
		if err == errQueueFull {
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
	pageNumber, offset, limit := 1, 0, srv.itemsPerPage
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
