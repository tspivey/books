package server

import (
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

var ErrBookNotReady = errors.New("book not ready")
var ErrQueueFull = errors.New("queue full")
var ErrConversionFailed = errors.New("conversion failed")

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
		if conversionErr != ErrBookNotReady {
			c.convertingMtx.Lock()
			delete(c.converting, bf.ID)
			c.convertingMtx.Unlock()
			return "", errors.Wrap(conversionErr, "Converting book")
		}
		return "", conversionErr
	}

	select {
	case c.fileCh <- bf:
		return "", ErrBookNotReady
	default:
		return "", ErrQueueFull
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
		c.converting[bookFile.ID] = ErrBookNotReady
		c.convertingMtx.Unlock()

		filename := path.Join(c.booksRoot, bookFile.CurrentFilename)
		newFile := path.Join(c.cacheDir, bookFile.Hash+".epub")
		cmd := exec.Command("ebook-convert", filename, newFile)
		err := cmd.Run()

		c.convertingMtx.Lock()
		if err != nil {
			c.converting[bookFile.ID] = err
		} else {
			delete(c.converting, bookFile.ID)
		}
		c.convertingMtx.Unlock()
	}
}

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
