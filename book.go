// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package books

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/pkg/xattr"
)

// Book represents a book in a library.
type Book struct {
	ID      int64
	Authors []string
	Title   string
	Series  string
	Files   []BookFile
}

// BookFile represents a file linked to a book.
type BookFile struct {
	ID               int64
	Extension        string
	Tags             []string
	Hash             string
	OriginalFilename string
	CurrentFilename  string
	FileMtime        time.Time
	FileSize         int64
	Source           string
}

// Filename retrieves a book's correct filename, based on the given output template.
func (bf *BookFile) Filename(tmpl *template.Template, book *Book) (string, error) {
	var fnBuff bytes.Buffer
	type FilenameTemplate struct {
		Book
		BookFile
		AuthorsShort string
	}
	ft := FilenameTemplate{*book, *bf, "Unknown"}
	if len(ft.Authors) == 1 {
		ft.AuthorsShort = ft.Authors[0]
	} else if len(ft.Authors) == 2 {
		ft.AuthorsShort = strings.Join(ft.Authors, " & ")
	} else if len(ft.Authors) > 2 {
		ft.AuthorsShort = strings.Join(ft.Authors[:2], " & ") + " & Others"
	}

	if err := tmpl.Execute(&fnBuff, ft); err != nil {
		return "", errors.Wrap(err, "Retrieve formatted filename for book")
	}
	return fnBuff.String(), nil
}

// CalculateHash calculates the hash of b.OriginalFilename and updates book.Hash.
// If a value is stored in the user.hash xattr, that value will be used instead of hashing the file's contents.
func (bf *BookFile) CalculateHash() error {
	if data, err := xattr.Get(bf.OriginalFilename, "user.hash"); err == nil {
		bf.Hash = string(data)
		return nil
	}
	fp, err := os.Open(bf.OriginalFilename)
	if err != nil {
		return errors.Wrap(err, "Calculate hash")
	}
	defer fp.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, fp)
	if err != nil {
		return errors.Wrap(err, "Calculate hash")
	}
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	bf.Hash = hash
	return nil
}

// ParseFilename creates a new Book given a filename and regular expression.
// The named groups author, title, series, and extension in the regular expression will map to their respective fields in the resulting book.
func ParseFilename(filename string, re *regexp.Regexp) (Book, bool) {
	result := Book{}
	bf := BookFile{}
	filename = path.Base(filename)
	mapping := re2map(filename, re)
	if mapping == nil {
		return result, false
	}
	for _, author := range strings.Split(mapping["author"], " & ") {
		result.Authors = append(result.Authors, strings.TrimSpace(author))
	}
	result.Title = mapping["title"]
	result.Series = mapping["series"]
	bf.Extension = mapping["ext"]
	result.Files = append(result.Files, bf)
	return result, true
}

// Escape replaces special characters in a filename with _.
func Escape(filename string) string {
	replacements := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}

	newFilename := filename
	for _, r := range replacements {
		newFilename = strings.Replace(newFilename, r, "_", -1)
	}
	return newFilename
}
