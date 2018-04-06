package books

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var initialSchema = `create table books (
id integer primary key,
created_on timestamp not null default (datetime()),
updated_on timestamp not null default (datetime()),
author text not null,
series text,
title text not null,
extension text not null,
tags text,
original_filename text not null,
filename text not null,
file_size integer not null,
file_mtime timestamp not null,
hash text not null unique,
regexp_name text not null,
template_override text,
source text
);

create virtual table books_fts using fts4 (author, series, title, extension, tags,  filename, source);
`

func init() {
	sql.Register("sqlite3async",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				conn.Exec("pragma synchronous=off", []driver.Value{})
				return nil
			},
		})
}

type Library struct {
	*sql.DB
}

func OpenLibrary(filename string) (*Library, error) {
	db, err := sql.Open("sqlite3async", filename)
	if err != nil {
		return nil, err
	}
	return &Library{db}, nil
}

func CreateLibrary(filename string) error {
	log.Printf("Creating library in %s\n", filename)
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return errors.Wrap(err, "Create library")
	}
	defer db.Close()

	_, err = db.Exec(initialSchema)
	if err != nil {
		return errors.Wrap(err, "Create library")
	}

	log.Printf("Library created in %s\n", filename)
	return nil
}

func (db *Library) ImportBook(book Book, move bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	rows, err := db.Query("select id from books where hash=?", book.Hash)
	if err != nil {
		tx.Rollback()
		return err
	}
	if rows.Next() {
		var id int64
		rows.Scan(&id)
		tx.Rollback()
		return errors.Errorf("Book already exists with id %d", id)
	}

	rows.Close()
	if rows.Err() != nil {
		tx.Rollback()
		return errors.Wrapf(err, "Searching for duplicate book by hash %s", book.Hash)
	}

	tags := strings.Join(book.Tags, "/")
	res, err := tx.Exec(`insert into books (author, series, title, extension, tags, original_filename, filename, file_size, file_mtime, hash, regexp_name, source)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.Author, book.Series, book.Title, book.Extension, tags, book.OriginalFilename, book.CurrentFilename, book.FileSize, book.FileMtime, book.Hash, book.RegexpName, book.Source)
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "Inserting book into the db")
	}

	id, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "Fetching new book ID")
	}
	book.Id = id

	res, err = tx.Exec(`insert into books_fts (docid, author, series, title, extension, tags,  filename, source)
	values (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, book.Author, book.Series, book.Title, book.Extension, tags, book.CurrentFilename, book.Source)
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "Indexing book for search")
	}

	err = db.copyBook(book, move)
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "Moving or copying book")
	}

	tx.Commit()
	log.Printf("Imported book: %s: %s, ID = %d", book.Author, book.Title, book.Id)

	return nil
}

func (db *Library) copyBook(book Book, move bool) error {
	root := viper.GetString("root")
	newName := book.CurrentFilename
	newPath := path.Join(root, newName)
	err := os.MkdirAll(path.Dir(newPath), 0755)
	if err != nil {
		return err
	}

	if move {
		err = moveFile(book.OriginalFilename, newPath)
	} else {
		err = copyFile(book.OriginalFilename, newPath)
	}
	if err != nil {
		return err
	}

	return nil
}

func (db *Library) Search(term string) ([]Book, error) {
	results := []Book{}
	rows, err := db.Query("select docid from books_fts where books_fts match ?", term)
	if err != nil {
		return results, err
	}
	var ids []int64
	var id int64
	for rows.Next() {
		rows.Scan(&id)
		ids = append(ids, id)
	}
	err = rows.Err()
	if err != nil {
		return results, err
	}
	results, err = db.GetBooksById(ids)
	if err != nil {
		return results, err
	}
	return results, err
}

func (db *Library) GetBooksById(ids []int64) ([]Book, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	results := []Book{}
	joined := strings.Repeat("?,", len(ids))
	joined = joined[:len(joined)-1]
	iids := make([]interface{}, 0)
	for _, id := range ids {
		iids = append(iids, id)
	}
	rows, err := db.Query("select id, author, series, tags, title, extension, filename from books where id in ("+joined+")", iids...)
	if err != nil {
		return results, errors.Wrap(err, "querying database for books by ID")
	}

	for rows.Next() {
		book := Book{}
		var tags string
		if err := rows.Scan(&book.Id, &book.Author, &book.Series, &tags, &book.Title, &book.Extension, &book.CurrentFilename); err != nil {
			return results, errors.Wrap(err, "scanning rows")
		}
		book.Tags = strings.Split(tags, "/")
		results = append(results, book)
	}
	if rows.Err() != nil {
		return results, errors.Wrap(err, "querying books by ID")
	}
	return results, nil
}

func copyFile(src, dst string) (e error) {
	fp, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "Copy file")
	}
	defer fp.Close()

	st, err := fp.Stat()
	if err != nil {
		return errors.Wrap(err, "Copy file")
	}

	fd, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "Copy file")
	}
	defer func() {
		if err := fd.Close(); err != nil {
			e = errors.Wrap(err, "Copy file")
		}
		_ = os.Chtimes(dst, time.Now(), st.ModTime())
	}()

	if _, err := io.Copy(fd, fp); err != nil {
		return errors.Wrap(err, "Copy file")
	}

	log.Printf("Copied %s to %s", src, dst)

	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		err = copyFile(src, dst)
		if err != nil {
			return err
		}
		err = os.Remove(src)
		if err != nil {
			log.Printf("Error removing %s: %s", src, err)
			return nil
		}

		log.Printf("Removed %s", src)
		return nil
	}

	log.Printf("Moved %s to %s", src, dst)
	return nil
}

func GetUniqueName(f string) string {
	i := 1
	ext := path.Ext(f)
	newName := f
	_, err := os.Stat(newName)
	for err == nil {
		newName = strings.TrimSuffix(f, ext) + " (" + strconv.Itoa(i) + ")" + ext
		i++
		_, err = os.Stat(newName)
	}
	return newName
}
