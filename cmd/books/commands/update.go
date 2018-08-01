// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"fmt"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a book",
	Long: `Updates a book from metadata.
`,
	Run: CPUProfile(updateFunc),
}

func init() {
	rootCmd.AddCommand(updateCmd)

}

func updateFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No book ID specified.\n")
		os.Exit(1)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Book ID must be a number.")
		os.Exit(1)
	}
	library, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s\n", err)
		os.Exit(1)
	}
	defer library.Close()

	bks, err := library.GetBooksByID([]int64{int64(id)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting books by ID: %s\n", err)
		os.Exit(1)
	} else if len(bks) == 0 {
		fmt.Fprintln(os.Stderr, "Book not found.\n")
		os.Exit(1)
	}

	book := bks[0]
	parser := &books.EpubMetadataParser{}
	files := []string{}
	for _, file := range book.Files {
		files = append(files, filepath.Join(booksRoot, file.CurrentFilename))
	}
	newBook, parsed := parser.Parse(files)
	if !parsed {
		fmt.Printf("Book %s - %s not updated.\n", joinNaturally("and", book.Authors), book.Title)
		os.Exit(0)
	}
	log.Printf("Updating book with new metadata: %s - %s\n", joinNaturally("and", newBook.Authors), newBook.Title)
	newBook.ID = book.ID
	err = library.UpdateBook(newBook)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating book: %s\n", err)
		os.Exit(1)
	}

}
