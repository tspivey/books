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

var compiled []*regexp.Regexp
var regexpNames []string
var outputTmpl *template.Template
var recursive bool
var metadataParsers []string
var metadataParserMap map[string]books.MetadataParser
var tagsRegexp = regexp.MustCompile(`^(.*)\(([^)]+)\)\s*$`)

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
	Run: CPUProfile(importFunc),
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringSliceP("metadata-parsers", "p", []string{}, "List of metadata parsers to use during import")
	importCmd.Flags().StringSliceP("regexp", "r", []string{"regexp"}, "List of regular expressions to use during import")
	importCmd.Flags().BoolP("move", "m", false, "Move files instead of copying them")
	importCmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Recurse into subdirectories")
	viper.BindPFlag("move", importCmd.Flags().Lookup("move"))
	viper.BindPFlag("default_metadata_parsers", importCmd.Flags().Lookup("metadata-parsers"))
	viper.BindPFlag("default_regexps", importCmd.Flags().Lookup("regexp"))
}

func importFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No files to import.\n")
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
	log.Printf("Using metadata parsers: %v\n", metadataParsers)
	outputTmplSrc := viper.GetString("output_template")
	var err error
	outputTmpl, err = template.New("filename").Funcs(funcMap).Parse(outputTmplSrc)
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
		if err := importBooks(path, recursive, library); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot import books from %s: %s; skipping\n", path, err)
			continue
		}
	}
}

// importBooks imports one or more books into the library.
// root may be either a file or directory.
func importBooks(root string, recursive bool, library *books.Library) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			log.Printf("Importing file %s:\n", path)
			if err := importBook(path, library); err != nil {
				log.Printf("Cannot import book from %s: %s; skipping\n", path, err)
			}
			return nil
		}

		if path != root && !recursive {
			return filepath.SkipDir
		}

		return nil
	})
}

// importBook imports a single book into the library.
func importBook(filename string, library *books.Library) error {
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

	err = bf.CalculateHash()
	if err != nil {
		return errors.Wrap(err, "Calculate book hash")
	}

	// CurrentFilename needs to be set so updateFilenames can copy/move the original file
	bf.CurrentFilename, err = filepath.Abs(bf.OriginalFilename)
	if err != nil {
		return errors.Wrap(err, "get absolute path")
	}
	book.Files = append(book.Files, bf)

	if err := library.ImportBook(book, outputTmpl, viper.GetBool("move")); err != nil {
		return errors.Wrap(err, "Import book into library")
	}

	return nil
}

// SplitTags takes an unsplit filename in the form "filename (tag1) (tag2)..."
// and returns the tags.
func splitTags(filename string) []string {
	// Match tags from the right first,
	// adding tags in reverse order until the last non 0 length match is the title.
	ext := path.Ext(filename)
	filename = strings.TrimSuffix(filename, ext)
	var tags = []string{}
	for {
		match := tagsRegexp.FindStringSubmatch(filename)
		if len(match) == 0 {
			break
		}
		filename = match[1]
		tags = append([]string{match[2]}, tags...)
	}
	return tags
}
