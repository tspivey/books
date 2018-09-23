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
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edits a book",
	Long: `Interactively edits a book.
`,
	Run: editFunc,
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
	editCmd.Flags().BoolP("file", "f", false, "Edit a file")
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
	cmdShow(&book, library, "")
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
		parse(&book, library, cmd)
	}

}

var cmdtable = map[string]func(book *books.Book, lib *books.Library, args string){
	"a":       cmdAuthors,
	"authors": cmdAuthors,
	"show":    cmdShow,
	"title":   cmdTitle,
	"series":  cmdSeries,
	"save":    cmdSave,
}

func parse(b *books.Book, lib *books.Library, cmd string) {
	lst := strings.SplitN(cmd, " ", 2)
	var args string
	if len(lst) == 1 {
		args = ""
	} else {
		args = lst[1]
	}
	fn, ok := cmdtable[lst[0]]
	if !ok {
		fmt.Println("Unknown command.")
		return
	}
	fn(b, lib, args)
}

func cmdShow(book *books.Book, library *books.Library, args string) {
	fmt.Println("Title: ", book.Title)
	fmt.Println("Authors: ", strings.Join(book.Authors, " & "))
	fmt.Println("Series: ", book.Series)
}

func cmdAuthors(book *books.Book, lib *books.Library, args string) {
	if args == "" {
		fmt.Fprintf(os.Stderr, "Usage: authors <authors>\n")
		return
	}
	newAuthors := strings.Split(args, " & ")
	book.Authors = newAuthors
}

func cmdTitle(book *books.Book, lib *books.Library, args string) {
	if args == "" {
		fmt.Fprintf(os.Stderr, "Usage: title <title>\n")
		return
	}
	book.Title = args
}

func cmdSeries(book *books.Book, lib *books.Library, args string) {
	if args == "" {
		fmt.Fprintf(os.Stderr, "Usage: series <series>\n")
		return
	}
	book.Series = args
}

func cmdSave(book *books.Book, lib *books.Library, args string) {
	err := lib.UpdateBook(*book, true)
	if bee, ok := err.(books.BookExistsError); ok {
		if args == "-m" {
			err := lib.MergeBooks([]int64{bee.BookID, book.ID})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error merging books: %v\n", err)
				return
			}
			fmt.Printf("Merged into %d\n", bee.BookID)
		} else {
			fmt.Printf("A duplicate book already exists, id: %d. To merge, type save -m.\n", bee.BookID)
			return
		}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error while updating book: %v\n", err)
		return
	}

}

func completer(book *books.Book, s string) []string {
	lst := strings.SplitN(s, " ", 2)
	if len(lst) < 1 {
		return []string{}
	}
	if lst[0] == "a" || lst[0] == "authors" {
		return []string{"authors " + strings.Join(book.Authors, " & ")}
	} else if lst[0] == "title" {
		return []string{"title " + book.Title}
	} else if lst[0] == "series" {
		return []string{"series " + book.Series}
	}
	return []string{}
}
