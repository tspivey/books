// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"log"
	"os"
	"strconv"
	"text/template"

	"fmt"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mergeCmd represents the merge command
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge books",
	Long: `Merges two or more books into the first one specified.
`,
	Run: CPUProfile(mergeFunc),
}

func init() {
	rootCmd.AddCommand(mergeCmd)

}

func mergeFunc(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "At least two book IDs must be specified.\n")
		os.Exit(1)
	}
	ids := []int64{}
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Book ID must be a number.")
			os.Exit(1)
		}
		ids = append(ids, int64(id))
	}

	library, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s\n", err)
		os.Exit(1)
	}
	defer library.Close()

	bks, err := library.GetBooksByID(ids)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting books by ID: %s\n", err)
		os.Exit(1)
	} else if len(bks) != len(ids) {
		fmt.Fprintln(os.Stderr, "All specified book IDs must exist.")
		os.Exit(1)
	}

	outputTmplSrc := viper.GetString("output_template")
	outputTmpl, err := template.New("filename").Funcs(funcMap).Parse(outputTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse output template: %s\n\n%s\n", err, outputTmplSrc)
		os.Exit(1)
	}

	var book books.Book
	for i := range bks {
		if bks[i].ID == ids[0] {
			book = bks[i]
			break
		}
	}

	log.Printf("Merging all books into %s (%d) by %s.\n", book.Title, book.ID, joinNaturally("and", book.Authors))
	if err := library.MergeBooks(ids, outputTmpl); err != nil {
		fmt.Fprintf(os.Stderr, "Error merging books: %s\n", err)
		os.Exit(1)
	}
}
