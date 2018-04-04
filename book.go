package books

import (
	"bytes"
	"path"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

var Version = "unset"

type Book struct {
	Author string
	Title  string
	Series string
	Format string
	Tags   []string
	Hash   string
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
	result.Format = mapping["ext"]
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
