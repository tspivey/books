// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"
	"github.com/tspivey/books"
	"github.com/tspivey/books/cmd/books/edit"
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edits a book",
	Run:   editFunc,
}

func init() {
	rootCmd.AddCommand(editCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// editCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// editCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	editCmd.Flags().BoolP("file", "f", false, "Specify a book  by one of its filenames")
}

func editFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: books edit <book id>\n")
		os.Exit(1)
	}
	useFile, err := cmd.Flags().GetBool("file")
	if err != nil {
		log.Fatal(err)
	}
	library, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening library: %s\n", err)
		os.Exit(1)
	}
	defer library.Close()

	var bookId int64
	if useFile {
		rootPath := booksRoot + string(os.PathSeparator)
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting absolute path: %s\n", err)
		}
		var fn string
		if strings.HasPrefix(absPath, rootPath) {
			fn = strings.TrimPrefix(absPath, rootPath)
		} else {
			fmt.Fprintf(os.Stderr, "Book not found.\n")
			os.Exit(1)
		}
		bookId, err = library.GetBookIDByFilename(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting book: %s\n", err)
			os.Exit(1)
		}
	} else {
		bookId, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid book ID.\n")
			os.Exit(1)
		}
	}

	books, err := library.GetBooksByID([]int64{int64(bookId)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting books by ID: %s", err)
		os.Exit(1)
	}
	if len(books) == 0 {
		fmt.Fprintf(os.Stderr, "Book not found.\n")
		os.Exit(1)
	}
	book := books[0]
	parser := edit.NewParser(&book, library)
	parser.RunCommand("show", "")
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetCompleter(func(s string) []string {
		return completer(&book, s)
	})
	for {
		cmd, err := line.Prompt(">")
		if err != nil {
			if err == liner.ErrPromptAborted || err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading line: %s\n", err)
			return
		}
		parse(parser, cmd)
	}

}

func parse(parser *edit.Parser, cmd string) {
	lst := strings.SplitN(cmd, " ", 2)
	var args string
	if len(lst) == 1 {
		args = ""
	} else {
		args = lst[1]
	}
	if parser.RunCommand(lst[0], args) == edit.ErrUnknownCommand {
		fmt.Println("Unknown command.")
		return
	}
}

func completer(book *books.Book, s string) []string {
	lst := strings.SplitN(s, " ", 2)
	if len(lst) < 1 {
		return []string{}
	}
	if lst[0] == "authors" {
		return []string{"authors " + strings.Join(book.Authors, " & ")}
	} else if lst[0] == "title" {
		return []string{"title " + book.Title}
	} else if lst[0] == "series" {
		return []string{"series " + book.Series}
	}
	return []string{}
}
