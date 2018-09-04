package books

import (
	"log"
	"path"
	"strings"

	"github.com/766b/mobi"
)

type MobiMetadataParser struct {
}

func (p *MobiMetadataParser) Parse(files []string) (book Book, parsed bool) {
	for _, file := range files {
		ext := path.Ext(strings.ToLower(file))
		if ext != ".azw3" && ext != ".mobi" {
			continue
		}
		f, err := mobi.NewReader(file)
		if err != nil {
			log.Printf("Error while opening mobi %s: %s", file, err)
			continue
		}
		defer f.Close()

		title := getFirstRecord(f, mobi.EXTH_UPDATEDTITLE)
		authors := getRecords(f, mobi.EXTH_AUTHOR)
		if title == "" || len(authors) == 0 {
			log.Printf("File %s has no title or author. Title: %v authors: %+v", file, title, authors)
			continue
		}
		book.Title = title
		book.Authors = authors
		return book, true
	}

	return
}

func getFirstRecord(r *mobi.MobiReader, record uint32) string {
	for _, rec := range r.Exth.Records {
		if rec.RecordType == record {
			return string(rec.Value)
		}
	}
	return ""
}

func getRecords(r *mobi.MobiReader, record uint32) (records []string) {
	for _, rec := range r.Exth.Records {
		if rec.RecordType == record {
			records = append(records, string(rec.Value))
		}
	}
	return
}
