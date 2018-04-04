package books

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type library struct {
	*sql.DB
}

func OpenLibrary(filename string) (*library, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	return &library{db}, nil
}

func (db *library) ImportBook(book Book) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rows, err := db.Query("select id from books where hash=?", book.Hash)
	if err != nil {
		tx.Rollback()
		return err
	}
	var id int64
	var exists bool
	for rows.Next() {
		exists = true
		rows.Scan(&id)
	}
	err = rows.Err()
	if exists {
		tx.Rollback()
		return errors.Errorf("Book already exists with id %d", id)
	}
	rows.Close()
	if rows.Err() != nil {
		tx.Rollback()
		return err
	}
	tags := strings.Join(book.Tags, "/")
	_, err = tx.Exec(`insert into books (author, series, title, extension, tags, original_filename, filename, file_size, file_mtime, hash, regexp_name, source)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.Author, book.Series, book.Title, book.Extension, tags, book.OriginalFilename, book.CurrentFilename, book.FileSize, book.FileMtime, book.Hash, book.RegexpName, book.Source)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}
