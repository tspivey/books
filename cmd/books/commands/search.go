// Copyright © 2018 Author

package commands

import (
	"books"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	lib, err := books.OpenLibrary(viper.GetString("db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s", err)
		os.Exit(1)
	}
	books, err := lib.Search(term)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while searching for books: %s\n", err)
		os.Exit(1)
	}
	for i, book := range books {
		tags := strings.Join(book.Tags, "/")
		fmt.Printf("%d. %s: %s (%s) (%s)\n", i+1, book.Author, book.Title, tags, book.Extension)
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
