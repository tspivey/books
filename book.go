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

var Version = "unset"
var tagsRegexp = regexp.MustCompile(`^(.*)\(([^)]+)\)\s*$`)

// Book represents a book in a library.
type Book struct {
	Id      int64
	Authors []string
	Title   string
	Series  string
	Files   []BookFile
}

type BookFile struct {
	Id               int64
	Extension        string
	Tags             []string
	Hash             string
	OriginalFilename string
	CurrentFilename  string
	FileMtime        time.Time
	FileSize         int64
	RegexpName       string
	Source           string
}

// Filename retrieves a book's correct filename, based on the given output template.
func (b *BookFile) Filename(tmpl *template.Template, book *Book) (string, error) {
	var fnBuff bytes.Buffer
	type FilenameTemplate struct {
		Book
		BookFile
	}
	ft := FilenameTemplate{*book, *b}
	if err := tmpl.Execute(&fnBuff, ft); err != nil {
		return "", errors.Wrap(err, "Retrieve formatted filename for book")
	}
	return fnBuff.String(), nil
}

// CalculateHash calculates the hash of b.OriginalFilename and updates book.Hash.
// If a value is stored in the user.hash xattr, that value will be used instead of hashing the file's contents.
func (b *BookFile) CalculateHash() error {
	if data, err := xattr.Get(b.OriginalFilename, "user.hash"); err == nil {
		b.Hash = string(data)
		return nil
	}
	fp, err := os.Open(b.OriginalFilename)
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
	b.Hash = hash
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

// SplitTitleAndTags takes an unsplit title in the form "title (tag1) (tag2)..."
// and returns the title and tags separately.
func SplitTitleAndTags(s string) (string, []string) {
	// Match tags from the right first,
	// adding tags in reverse order until the last non 0 length match is the title.
	var tags = []string{}
	for {
		match := tagsRegexp.FindStringSubmatch(s)
		if len(match) == 0 {
			break
		}
		s = match[1]
		tags = append([]string{match[2]}, tags...)
	}
	return strings.Trim(s, " "), tags
}

// re2map returns a map of named groups to their matches.
// Example:
//     regexp: ^(?P<first>\w+) (?P<second>\w+)$
//     string: hello world
//     result: first => hello, second => world
func re2map(s string, r *regexp.Regexp) map[string]string {
	rmap := make(map[string]string)
	matches := r.FindStringSubmatch(s)
	if len(matches) == 0 {
		return nil
	}

	names := r.SubexpNames()
	for i, n := range names {
		if i == 0 {
			continue
		}
		rmap[n] = matches[i]
	}

	return rmap
}
