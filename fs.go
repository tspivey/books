package books

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// TruncateFilename truncates each path segment to 255 bytes.
// The filename portion is truncated to 250 bytes to leave room for getUniqueFilename,
// and bytes are removed just before the extension.
func TruncateFilename(fn string) string {
	var lst []string
	dirs, fn := path.Split(fn)
	if dirs != "" {
		dirs = strings.TrimRight(dirs, string(os.PathSeparator))
		lst = strings.Split(dirs, string(os.PathSeparator))
	}
	for i, f := range lst {
		if len(f) <= 255 {
			continue
		}

		lst[i] = f[:255]
	}

	if len(fn) > 250 {
		ext := path.Ext(fn)
		nameLen := 250 - len(ext)
		fn = fn[:nameLen] + ext
	}
	lst = append(lst, fn)
	return filepath.Join(lst...)
}

// moveOrCopyFile moves or copies a file from origName to newName.
// All necessary directories to make the destination valid will be created.
func moveOrCopyFile(origName, newName string, move bool) error {
	err := os.MkdirAll(path.Dir(newName), 0755)
	if err != nil {
		return errors.Wrap(err, "create destination directory")
	}

	if move {
		return moveFile(origName, newName)
	}
	return copyFile(origName, newName)
}

// copyFile copies a file from src to dst, setting dst's modified time to that of src.
func copyFile(src, dst string) (e error) {
	fp, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "Copy file")
	}
	defer fp.Close()

	st, err := fp.Stat()
	if err != nil {
		return errors.Wrap(err, "Copy file")
	}

	fd, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "create destination file")
	}
	defer func() {
		if err := fd.Close(); err != nil {
			e = errors.Wrap(err, "close destination file")
		}
		if err := os.Chtimes(dst, time.Now(), st.ModTime()); err != nil {
			log.Printf("Error updating times of %s: %s", dst, err)
		}
	}()

	if _, err := io.Copy(fd, fp); err != nil {
		return errors.Wrap(err, "Copy file")
	}

	log.Printf("Copied %s to %s", src, dst)

	return nil
}

// moveFile moves a file from src to dst.
// First, moveFile will attempt to rename the file,
// and if that fails, it will perform a copy and delete.
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

		log.Printf("Moved %s to %s (copy/delete)", src, dst)
		return nil
	}

	log.Printf("Moved %s to %s", src, dst)
	return nil
}

// GetUniqueName checks to see if a file named f already exists, and if so, finds a unique name.
func GetUniqueName(f string) (string, error) {
	i := 1
	ext := path.Ext(f)
	newName := f
	_, err := os.Stat(newName)
	for err == nil {
		newName = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(f, ext), i, ext)
		i++
		_, err = os.Stat(newName)
	}
	if !os.IsNotExist(err) {
		return newName, err
	}
	return newName, nil
}
