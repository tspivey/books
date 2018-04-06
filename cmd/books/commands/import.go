// Copyright © 2018 Author

package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"fmt"

	"github.com/pkg/errors"
	"github.com/tspivey/books"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var compiled []*regexp.Regexp
var regexpNames []string
var outputTmpl *template.Template
var recursive bool

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a book into the library",
	Long: `Import books into the library from the specified directory.

You can pass one or more files or directories as arguments to import.
If a directory is passed, the books in that directory will be imported,
but the books in its subdirectories will not be, unless --recursive is set.

Each file will be matched against the list of regular expressions in order, and will be imported according to the first match.
The following named groups will be recognized: author, series, title, and ext.
Your files will be named according to the output template in the config file,
or the template override set in the library.`,
	Run: CpuProfile(importFunc),
}

func init() {
	rootCmd.AddCommand(importCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	importCmd.Flags().StringSliceP("regexp", "r", []string{}, "List of regular expressions to use during import")
	importCmd.Flags().BoolP("move", "m", false, "Move files instead of copying them")
	importCmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Recurse into subdirectories")
	viper.BindPFlag("move", importCmd.Flags().Lookup("move"))
	viper.BindPFlag("default_regexps", importCmd.Flags().Lookup("regexp"))
}

func importFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No files to import.")
		os.Exit(1)
	}

	res := viper.GetStringSlice("default_Regexps")
	if len(res) == 0 {
		fmt.Fprintf(os.Stderr, "Either -r must be specified, or default_regexps must be set in the configuration file.\n")
		os.Exit(1)
	}

	for _, v := range res {
		reString := viper.GetString("regexps." + v)
		if reString == "" {
			fmt.Fprintf(os.Stderr, "Regexp %s not found in config\n", v)
			os.Exit(1)
		}
		regexpNames = append(regexpNames, v)
		c, err := regexp.Compile(reString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot compile regular expression %s: %s\n", v, err)
			os.Exit(1)
		}
		compiled = append(compiled, c)
	}

	outputTmplSrc := viper.GetString("output_template")
	var err error
	outputTmpl, err = template.New("filename").Funcs(template.FuncMap{"ToUpper": strings.ToUpper}).Parse(outputTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse output template: %s\n\n%s\n", err, outputTmplSrc)
		os.Exit(1)
	}

	library, err := books.OpenLibrary(libraryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening Library: %s\n", err)
		os.Exit(1)
	}
	defer library.Close()

	for _, path := range args {
		fmt.Printf("Importing %s:\n", path)
		if err := importBooks(path, recursive, library); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot import books from %s: %s; skipping\n", path, err)
			continue
		}
	}
}

func importBooks(root string, recursive bool, library *books.Library) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fmt.Printf("Importing file %s:\n", path)
			if err := importBook(path, library); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot import book from %s: %s; skipping\n", path, err)
			}
			return nil
		}

		if path != root && !recursive {
			return filepath.SkipDir
		}

		return nil
	})
}

func importBook(path string, library *books.Library) error {
	var book books.Book
	var matched bool
	for i, c := range compiled {
		if book, matched = books.ParseFilename(path, c); matched {
			book.RegexpName = regexpNames[i]
			book.OriginalFilename = path
			break
		}
	}
	if !matched {
		return errors.Errorf("No regular expression matched %s", path)
	}

	title, tags := books.SplitTitleAndTags(book.Title)
	book.Title = title
	book.Tags = tags

	fi, err := os.Stat(path)
	if err != nil {
		return errors.Wrap(err, "Get file info for book")
	}
	book.FileSize = fi.Size()
	book.FileMtime = fi.ModTime()

	err = book.CalculateHash()
	if err != nil {
		return errors.Wrap(err, "Calculate book hash")
	}

	s, err := book.Filename(outputTmpl)
	if err != nil {
		return errors.Wrap(err, "Calculate output filename for book")
	}
	book.CurrentFilename = books.GetUniqueName(s)
	fmt.Printf("Using regexp name: %s\n", book.RegexpName)

	if err := library.ImportBook(book, viper.GetBool("move")); err != nil {
		return errors.Wrap(err, "Import book into library")
	}

	return nil
}
