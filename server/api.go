package server

import (
	"time"

	"github.com/tspivey/books"
)

type apiError struct {
	Error string `json:"error"`
}

// Book represents a book in a library.
type Book struct {
	ID      int64      `json:"id"`
	Authors []string   `json:"authors"`
	Title   string     `json:"title"`
	Series  string     `json:"series"`
	Files   []BookFile `json:"files"`
}

// BookFile represents a file linked to a book.
type BookFile struct {
	ID               int64     `json:"id"`
	Extension        string    `json:"extension"`
	Tags             []string  `json:"tags"`
	Hash             string    `json:"hash"`
	OriginalFilename string    `json:"original_filename"`
	Filename         string    `json:"filename"`
	Mtime            time.Time `json:"mtime"`
	Size             int64     `json:"size"`
}

type updateBook struct {
	Book            Book `json:"book"`
	OverwriteSeries bool `json:"overwrite_series"`
}

type success struct {
	Success string `json:"success"`
}

func bookToModel(book books.Book) Book {
	modelFiles := make([]BookFile, 0)
	for _, file := range book.Files {
		newFile := BookFile{
			ID:               file.ID,
			Extension:        file.Extension,
			Tags:             file.Tags,
			Hash:             file.Hash,
			OriginalFilename: file.OriginalFilename,
			Filename:         file.CurrentFilename,
			Mtime:            file.FileMtime,
			Size:             file.FileSize,
		}
		if newFile.Tags == nil {
			newFile.Tags = make([]string, 0)
		}
		modelFiles = append(modelFiles, newFile)
	}
	newBook := Book{
		ID:      book.ID,
		Authors: book.Authors,
		Title:   book.Title,
		Series:  book.Series,
		Files:   modelFiles,
	}
	if newBook.Authors == nil {
		newBook.Authors = make([]string, 0)
	}
	return newBook
}

func modelToBook(modelBook Book) books.Book {
	files := make([]books.BookFile, 0)
	for _, file := range modelBook.Files {
		newFile := books.BookFile{
			ID:               file.ID,
			Extension:        file.Extension,
			Tags:             file.Tags,
			Hash:             file.Hash,
			OriginalFilename: file.OriginalFilename,
			CurrentFilename:  file.Filename,
			FileMtime:        file.Mtime,
			FileSize:         file.Size,
		}
		files = append(files, newFile)
	}
	newBook := books.Book{
		ID:      modelBook.ID,
		Authors: modelBook.Authors,
		Title:   modelBook.Title,
		Series:  modelBook.Series,
		Files:   files,
	}
	return newBook
}
