package books

import (
	"log"
	"path"
	"regexp"
	"strings"

	"github.com/kapmahc/epub"
)

// A MetadataParser is used to parse the metadata for a book from a list of BookFiles.
// As some implementations will use info from BookFiles to make parsing decisions,
// it is best to populate these as much as possible before passing them to Parse.
type MetadataParser interface {
	Parse(files []string) (book Book, parsed bool)
}

// A RegexpMetadataParser parses book metadata using each BookFile’s OriginalFilename.
// The first regular expression to match any files will be used, and if it matches multiple files,
// the authors/series/title combination that occurs most often will be used to set these fields on the book.
// Each BookFile will still have their tags and extension set individually.
// Each file’s extension will be trimmed before regular expressions are tested.
// If a file doesn’t match the used regular expression, it will be included with its extension and no tags.
// Regexps and RegexpNames must match.
type RegexpMetadataParser struct {
	Regexps     []*regexp.Regexp
	RegexpNames []string
}

// Parse parses a list of files using regexps.
func (p *RegexpMetadataParser) Parse(files []string) (book Book, parsed bool) {
	if len(p.Regexps) != len(p.RegexpNames) {
		log.Printf("RegexpMetadataParser: lengths of regexps and names are not equal")
		return
	}
	for i, c := range p.Regexps {
		for _, file := range files {
			filename := path.Base(file)
			mapping := re2map(filename, c)
			if mapping == nil {
				continue
			}
			log.Printf("Parsed metadata from file %s using regexp name %s", file, p.RegexpNames[i])
			for _, author := range strings.Split(mapping["author"], " & ") {
				book.Authors = append(book.Authors, strings.TrimSpace(author))
			}
			book.Title = mapping["title"]
			book.Series = mapping["series"]
			return book, true
		}
	}
	return
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

// EpubMetadataParser parses files using EPUB metadata.
type EpubMetadataParser struct{}

// Parse parses a list of files using EPUB metadata.
func (*EpubMetadataParser) Parse(files []string) (book Book, parsed bool) {
	for _, file := range files {
		if path.Ext(strings.ToLower(file)) != ".epub" {
			continue
		}
		f, err := epub.Open(file)
		if err != nil {
			log.Printf("Error while opening epub %s: %s", file, err)
			continue
		}

		m := f.Opf.Metadata
		if len(m.Title) == 0 || m.Title[0] == "" {
			f.Close()
			continue
		}
		book.Title = m.Title[0]

		book.Authors = make([]string, 0)
		for _, author := range m.Creator {
			if author.Data != "" {
				book.Authors = append(book.Authors, author.Data)
			}
		}
		if len(book.Authors) == 0 {
			f.Close()
			continue
		}
		f.Close()

		return book, true
	}

	return
}
