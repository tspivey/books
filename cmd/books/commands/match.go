// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"log"
	"os"
	"path"
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

// matchCmd represents the match command
var matchCmd = &cobra.Command{
	Use:   "match",
	Short: "Find duplicates based on author and title",
	Long: `Find duplicate books based on parsed author and title.
`,
	Run: CPUProfile(matchFunc),
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("default_metadata_parsers", cmd.Flags().Lookup("metadata-parsers"))
		viper.BindPFlag("default_regexps", cmd.Flags().Lookup("regexp"))
	},
}

func init() {
	rootCmd.AddCommand(matchCmd)

	matchCmd.Flags().StringSliceP("metadata-parsers", "p", []string{}, "List of metadata parsers to use during search")
	matchCmd.Flags().StringSliceP("regexp", "r", []string{"regexp"}, "List of regular expressions to use during search")
	matchCmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Recurse into subdirectories")
}

func matchFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No files to search.\n")
		os.Exit(1)
	}

	// Get regular expressions by their names and compile them.
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

	metadataParserMap = make(map[string]books.MetadataParser)
	metadataParserMap["regexp"] = &books.RegexpMetadataParser{
		Regexps:     compiled,
		RegexpNames: regexpNames,
	}
	metadataParserMap["epub"] = &books.EpubMetadataParser{}
	metadataParsers = viper.GetStringSlice("default_metadata_parsers")
	for _, name := range metadataParsers {
		if _, ok := metadataParserMap[name]; !ok {
			fmt.Fprintf(os.Stderr, "Metadata parser %s not found.\n", name)
			os.Exit(1)
		}
	}
	if len(metadataParsers) == 0 {
		fmt.Fprintf(os.Stderr, "No metadata parsers defined.\n")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Using metadata parsers: %v\n", metadataParsers)
	outputTmplSrc := viper.GetString("output_template")
	var err error
	outputTmpl, err = template.New("filename").Funcs(template.FuncMap{"ToUpper": strings.ToUpper, "join": strings.Join, "escape": books.Escape}).Parse(outputTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse output template: %s\n\n%s\n", err, outputTmplSrc)
		os.Exit(1)
	}

	library, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening Library: %s\n", err)
		os.Exit(1)
	}
	defer library.Close()

	for _, path := range args {
		if err := searchDupes(path, recursive, library); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot search books from %s: %s; skipping\n", path, err)
			continue
		}
	}
}

// searchDupes searches for one or more duplicate books.
// root may be either a file or directory.
func searchDupes(root string, recursive bool, library *books.Library) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if err := searchDupe(path, library); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot search book from %s: %s; skipping\n", path, err)
			}
			return nil
		}

		if path != root && !recursive {
			return filepath.SkipDir
		}

		return nil
	})
}

// searchDupe searches for a single duplicate book.
func searchDupe(filename string, library *books.Library) error {
	fi, err := os.Stat(filename)
	if err != nil {
		return errors.Wrap(err, "Get file info for book")
	}

	tags := splitTags(filename)
	ext := path.Ext(filename)
	var book books.Book
	var matched bool
	for _, parserName := range metadataParsers {
		if book, matched = metadataParserMap[parserName].Parse([]string{filename}); matched {
			log.Printf("Matched metadata parser: %s", parserName)
			break
		}
	}
	if !matched {
		return errors.Errorf("No metadata parser matched %s", filename)
	}

	bf := books.BookFile{Tags: tags, OriginalFilename: filename}
	bf.FileSize = fi.Size()
	bf.FileMtime = fi.ModTime()
	bf.Extension = strings.TrimPrefix(ext, ".")

	s, err := bf.Filename(outputTmpl, &book)
	if err != nil {
		return errors.Wrap(err, "Calculate output filename for book")
	}
	s = books.TruncateFilename(s)
	newFilename, err := books.GetUniqueName(filepath.Join(booksRoot, s))
	if err != nil {
		return errors.Wrap(err, "get unique filename")
	}
	bf.CurrentFilename, err = filepath.Rel(booksRoot, newFilename)
	if err != nil {
		return errors.Wrap(err, "get new book filename")
	}
	bf.CurrentFilename = strings.Replace(bf.CurrentFilename, string(filepath.Separator), "/", -1)
	book.Files = append(book.Files, bf)

	id, found, err := library.GetBookIDByTitleAndAuthors(book.Title, book.Authors)
	if err != nil {
		return errors.Wrap(err, "Search for duplicate book")
	}
	if found {
		bks, _ := library.GetBooksByID([]int64{id})
		bk := bks[0]
		_ = bk
		fmt.Println(filename)
		//fmt.Printf("%s", booksRoot+"/"+bk.Files[0].CurrentFilename)
		//fmt.Println("\n")
	}

	return nil
}
