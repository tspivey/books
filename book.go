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

type Book struct {
Id int64
	Author           string
	Title            string
	Series           string
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

func (b Book) Filename(tmpl *template.Template) (string, error) {
	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, b); err != nil {
		return "", errors.Wrapf(err, "Cannot get filename for book: %+v", b)
	}
	return tpl.String(), nil
}

func ParseFilename(filename string, re *regexp.Regexp) (Book, bool) {
	result := Book{}
	filename = path.Base(filename)
	mapping := re2map(filename, re)
	if mapping == nil {
		return result, false
	}
	result.Author = mapping["author"]
	result.Title = mapping["title"]
	result.Series = mapping["series"]
	result.Extension = mapping["ext"]
	return result, true
}

var tagsRegexp = regexp.MustCompile(`^(.*)\(([^)]+)\)\s*$`)

func SplitTitleAndTags(s string) (string, []string) {
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

func re2map(s string, r *regexp.Regexp) map[string]string {
	rmap := make(map[string]string)
	b := []byte(s)
	matches := r.FindSubmatch(b)
	if len(matches) == 0 {
		return nil
	}
	names := r.SubexpNames()
	for i, n := range names {
		if i == 0 {
			continue
		}
		v := string(matches[i])
		rmap[n] = v
	}
	return rmap
}

func (b *Book) CalculateHash() error {
	if data, err := xattr.Get(b.OriginalFilename, "user.hash"); err == nil {
		b.Hash = string(data)
		return nil
	}
	fp, err := os.Open(b.OriginalFilename)
	if err != nil {
		return errors.Wrap(err, "Cannot calculate hash")
	}
	defer fp.Close()
	hasher := sha256.New()
	_, err = io.Copy(hasher, fp)
	if err != nil {
		return errors.Wrap(err, "Cannot calculate hash")
	}
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	b.Hash = hash
	return nil
}
