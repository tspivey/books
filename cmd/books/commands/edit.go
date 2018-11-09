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
	"text/template"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	var bookID int64
	if useFile {
		rootPath := booksRoot + string(os.PathSeparator)
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting absolute path: %s\n", err)
			os.Exit(1)
		}
		var fn string
		if strings.HasPrefix(absPath, rootPath) {
			fn = strings.TrimPrefix(absPath, rootPath)
		} else {
			fmt.Fprintf(os.Stderr, "Book not found.\n")
			os.Exit(1)
		}
		bookID, err = library.GetBookIDByFilename(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting book: %s\n", err)
			os.Exit(1)
		}
	} else {
		bookID, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid book ID.\n")
			os.Exit(1)
		}
	}

	foundBooks, err := library.GetBooksByID([]int64{int64(bookID)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting books by ID: %s", err)
		os.Exit(1)
	}
	if len(foundBooks) == 0 {
		fmt.Fprintf(os.Stderr, "Book not found.\n")
		os.Exit(1)
	}
	book := foundBooks[0]
	outputTmplSrc := viper.GetString("output_template")
	outputTmpl, err := template.New("filename").Funcs(funcMap).Parse(outputTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse output template: %s\n\n%s\n", err, outputTmplSrc)
		os.Exit(1)
	}
	parser := edit.NewParser(&book, library, outputTmpl)
	parser.RunCommand("show", "")
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetCompleter(parser.Completer)
	for {
		cmd, err := line.Prompt(">")
		if err != nil {
			if err == liner.ErrPromptAborted || err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading line: %s\n", err)
			return
		}
		if err := parse(parser, cmd); err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "Error running command: %s\n", err)
			return
		}
	}

}

func parse(parser *edit.Parser, cmd string) error {
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	lst := strings.SplitN(cmd, " ", 2)
	var args string
	if len(lst) > 1 {
		args = lst[1]
	}
	err := parser.RunCommand(lst[0], args)
	if err == edit.ErrUnknownCommand {
		fmt.Println("Unknown command.")
		return nil
	}

	return err
}
