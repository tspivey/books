package books

import (
	"database/sql"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
"time"
"log"

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
	_, err = tx.Exec(`insert into books (author, series, title, extension, tags, original_filename, filename, file_size, file_mtime, hash, regexp_name, source)
	values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.Author, book.Series, book.Title, book.Extension, tags, book.OriginalFilename, book.CurrentFilename, book.FileSize, book.FileMtime, book.Hash, book.RegexpName, book.Source)
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
