package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/tspivey/books"
)

func (srv *Server) getBookHandler(w http.ResponseWriter, r *http.Request) {
	bookID := mux.Vars(r)["id"]
	id, err := strconv.Atoi(bookID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	bookList, err := srv.lib.GetBooksByID([]int64{int64(id)})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error getting book by ID: %v", err)
		return
	}
	if len(bookList) == 0 {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, Error{"no books"})
		return
	}
	model := BookToModel(bookList[0])
	writeJSON(w, model)
}

func (srv *Server) updateBookHandler(w http.ResponseWriter, r *http.Request) {
	var ub UpdateBook
	if !readPostedJSON(w, r, &ub) {
		return
	}
	book := ModelToBook(ub.Book)
	if book.ID == 0 {
		writeJSON(w, Error{"no book ID"})
		return
	}
	if book.Title == "" || len(book.Authors) == 0 {
		writeJSON(w, Error{"no title/authors"})
		return
	}
	err := srv.lib.UpdateBook(book, srv.outputTemplate, ub.OverwriteSeries)
	if bee, ok := err.(books.BookExistsError); ok {
		msg := fmt.Sprintf("Book exists: %d", bee.BookID)
		writeJSON(w, Error{msg})
		return
	}
	if err == books.ErrBookNotFound {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, Error{"book not found"})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Error updating book %d: %v", book.ID, err)
		writeJSON(w, Error{"internal server error"})
		return
	}
	writeJSON(w, Success{"updated"})
}

func (srv *Server) mergeHandler(w http.ResponseWriter, r *http.Request) {
	var ids []int64
	if !readPostedJSON(w, r, &ids) {
		return
	}
	if err := srv.lib.MergeBooks(ids, srv.outputTemplate); err != nil {
		log.Printf("error merging books: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, Error{"error merging books"})
		return
	}
	writeJSON(w, Success{"merged"})
}

func (srv *Server) apiSearchHandler(w http.ResponseWriter, r *http.Request) {
	term, ok := r.URL.Query()["term"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, Error{"no term specified"})
		return
	}
	bookList, err := srv.lib.Search(term[0])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error searching for book: %v", err)
		return
	}
	newList := []Book{}
	for i := range bookList {
		newList = append(newList, BookToModel(bookList[i]))
	}
	writeJSON(w, newList)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, string(b))
}

func readPostedJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		log.Printf("Error reading POST data: %v", err)
		return false
	}
	if err := r.Body.Close(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Error closing body: %v", err)
		return false
	}
	if err := json.Unmarshal(body, &v); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, Error{err.Error()})
		return false
	}
	return true
}
