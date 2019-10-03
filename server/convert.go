package server

import (
	"log"
	"os"
	"os/exec"
	"path"
	"sync"

	"github.com/pkg/errors"

	"github.com/tspivey/books"
)

// BookConverter converts a book to epub.
type BookConverter interface {
	Convert(bf books.BookFile) (string, error)
	Close()
}

// calibreBookConverter converts a book to epub using calibre.
type calibreBookConverter struct {
	convertingMtx sync.Mutex
	converting    map[int64]error // Holds book conversion status
	fileCh        chan books.BookFile
	booksRoot     string
	cacheDir      string
	closed        bool
}

var errBookNotReady = errors.New("book not ready")
var errQueueFull = errors.New("queue full")

func (c *calibreBookConverter) Convert(bf books.BookFile) (string, error) {
	if c.closed {
		return "", errors.New("book converter closed")
	}
	epubFn := path.Join(c.cacheDir, bf.Hash+".epub")
	_, err := os.Stat(epubFn)
	if err == nil {
		return epubFn, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	// The converted epub doesn't exist. Tell a worker to convert it if one isn't already doing so.
	c.convertingMtx.Lock()
	conversionErr, converting := c.converting[bf.ID]
	c.convertingMtx.Unlock()
	if converting {
		if conversionErr != errBookNotReady {
			c.convertingMtx.Lock()
			delete(c.converting, bf.ID)
			c.convertingMtx.Unlock()
			return "", errors.Wrap(conversionErr, "Converting book")
		}
		return "", conversionErr
	}

	select {
	case c.fileCh <- bf:
		return "", errBookNotReady
	default:
		return "", errQueueFull
	}
}

func (c *calibreBookConverter) Close() {
	c.closed = true
	close(c.fileCh)
}

// work listens on c.fileCh for files to convert to epub.
func (c *calibreBookConverter) work() {
	for bookFile := range c.fileCh {
		c.convertingMtx.Lock()
		c.converting[bookFile.ID] = errBookNotReady
		c.convertingMtx.Unlock()

		filename := path.Join(c.booksRoot, bookFile.HashPath())
		tmpFile := path.Join(c.cacheDir, bookFile.Hash+"."+bookFile.Extension)
		newFile := path.Join(c.cacheDir, bookFile.Hash+".epub")
		err := os.Symlink(filename, tmpFile)
		if err == nil {
			cmd := exec.Command("ebook-convert", tmpFile, newFile)
			err = cmd.Run()
			if err := os.Remove(tmpFile); err != nil {
				log.Printf("Unable to remove %s: %v", tmpFile, err)
			}
		}

		c.convertingMtx.Lock()
		if err != nil {
			log.Printf("%v", err)
			c.converting[bookFile.ID] = err
		} else {
			delete(c.converting, bookFile.ID)
		}
		c.convertingMtx.Unlock()
	}
}

// NewCalibreBookConverter creates a new BookConverter which uses calibre.
func NewCalibreBookConverter(booksRoot, cacheDir string, numWorkers int) BookConverter {
	converter := &calibreBookConverter{
		fileCh:     make(chan books.BookFile),
		converting: make(map[int64]error),
		booksRoot:  booksRoot,
		cacheDir:   cacheDir,
	}

	for i := 0; i < numWorkers; i++ {
		go converter.work()
	}

	return converter
}
