package books

import (
	"database/sql"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
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

func (db *library) ImportBook(book Book, move bool) error {
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
	res, err := tx.Exec(`insert into books (author, series, title, extension, tags, original_filename, filename, file_size, file_mtime, hash, regexp_name, source)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.Author, book.Series, book.Title, book.Extension, tags, book.OriginalFilename, book.CurrentFilename, book.FileSize, book.FileMtime, book.Hash, book.RegexpName, book.Source)
	if err != nil {
		tx.Rollback()
		return err
	}
	id, err = res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return err
	}
	res, err = tx.Exec(`insert into books_fts (docid, author, series, title, extension, tags,  filename, source)
	values (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, book.Author, book.Series, book.Title, book.Extension, tags, book.CurrentFilename, book.Source)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = db.copyBook(book, move)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (db *library) copyBook(book Book, move bool) error {
	root := viper.GetString("root")
	newName := book.CurrentFilename
	newPath := path.Join(root, newName)
	err := os.MkdirAll(path.Dir(path.Join(root, newName)), 0755)
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

func (db *library) Search(term string) ([]Book, error) {
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

func (db *library) GetBooksById(ids []int64) ([]Book, error) {
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
	rows, err := db.Query("select id, author, series, tags, title, extension from books where id in ("+joined+")", iids...)
	if err != nil {
		return results, errors.Wrap(err, "querying database for books by ID")
	}

	for rows.Next() {
		book := Book{}
		var tags string
		if err := rows.Scan(&book.Id, &book.Author, &book.Series, &tags, &book.Title, &book.Extension); err != nil {
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
		return err
	}
	defer fp.Close()
	st, err := fp.Stat()
	if err != nil {
		return err
	}
	fd, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := fd.Close(); err != nil {
			e = err
		}
		_ = os.Chtimes(dst, time.Now(), st.ModTime())
	}()
	if _, err := io.Copy(fd, fp); err != nil {
		return err
	}
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
	}
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
