// Copyright © 2018 Author

package commands

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search the library",
	Long: `Search the library.
By default, all fields are searched. This can be overridden with field:value.
Supported fields: author, series, title, tags, extension.
`,
	Run: CpuProfile(searchRun),
}

func searchRun(cmd *cobra.Command, args []string) {
	term := strings.Join(args, " ")
	lib, err := books.OpenLibrary(libraryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s", err)
		os.Exit(1)
	}
	books, err := lib.Search(term)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while searching for books: %s\n", err)
		os.Exit(1)
	}

	resultTmplSrc := `{{range $i, $v := . -}}
{{inc $i}}: {{$v.Author}} - {{$v.Title -}}
{{if $v.Series}} [{{$v.Series}}]{{end -}}
{{if $v.Tags}}({{range $i, $v := .Tags -}}
{{if $i}}, {{end -}}
{{$v}}{{end}}){{end -}}
.{{$v.Extension}}
{{end}}`
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}

	tmpl, err := template.New("search_result").Funcs(funcMap).Parse(resultTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing template: %s\n", err)
		os.Exit(1)
	}
	err = tmpl.Execute(os.Stdout, books)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing template: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(searchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// searchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
