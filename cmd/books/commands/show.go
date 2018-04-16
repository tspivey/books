// Copyright © 2018 Author

package commands

import (
	"fmt"
	"os"
	"strconv"
	"text/template"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show BOOK_ID",
	Short: "Show details for a book",
	Long: `Show details and files for a book by its ID.

Use search to find the ID of the book you want to show.`,
	Run: CpuProfile(showRun),
}

func showRun(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No book ID specified.")
		os.Exit(1)
	}

	bookId, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Book ID must be a number.")
		os.Exit(1)
	}

	lib, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s\n", err)
		os.Exit(1)
	}

	books, err := lib.GetBooksById([]int64{int64(bookId)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while getting book by id: %s\n", err)
		os.Exit(1)
	}

	if len(books) == 0 {
		fmt.Fprintln(os.Stderr, "No book found")
		os.Exit(1)
	} else if len(books) > 1 {
		fmt.Fprintln(os.Stderr, "More than one book found; exiting")
		os.Exit(1)
	}

	bookDetailsTmplSrc := `{{joinNaturally "and" .Authors}} - {{.Title }}
{{if .Series}}Series: {{.Series}}
{{end }}
{{ if .Files}}{{range .Files -}}
{{ .Extension -}}
: {{if .Tags}}({{range $i, $v := .Tags -}}
{{if $i}}, {{end -}}
{{ $v }}{{end}}){{end -}}
 ({{ .Id }})
{{ end }}
{{ else }}No files available for this book{{ end }}`

	tmpl, err := template.New("book_details").Funcs(funcMap).Parse(bookDetailsTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing template: %s\n", err)
		os.Exit(1)
	}

	err = tmpl.Execute(os.Stdout, books[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing template: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(showCmd)
}
