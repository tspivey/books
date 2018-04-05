// Copyright © 2018 Author

package commands

import (
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"

	"books"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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
	importCmd.Flags().StringSliceP("regexp", "r", []string{}, "Regexps to use during import")
	viper.BindPFlag("default_regexps", importCmd.Flags().Lookup("regexp"))
}

func importFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		log.Fatal("Filename to import is required.")
	}
	res := viper.GetStringSlice("default_Regexps")
	if len(res) == 0 {
		fmt.Fprintf(os.Stderr, "Either -r must be specified, or default_regexps must be set in the configuration file.\n")
		os.Exit(1)
	}
	var compiled []*regexp.Regexp
	var regexpNames []string
	for _, v := range res {
		reString := viper.GetString("regexps." + v)
		if reString == "" {
			log.Fatalf("Regexp %s not found in config", v)
		}
		regexpNames = append(regexpNames, v)
		c, err := regexp.Compile(reString)
		if err != nil {
			log.Fatalf("Cannot compile regular expression %s: %s", v, err)
		}
		compiled = append(compiled, c)
	}
	library, err := books.OpenLibrary(viper.GetString("db"))
	if err != nil {
		log.Fatal("Error opening Library: ", err)
	}
	defer library.Close()
	for _, f := range args {
		var book books.Book
		var parsed bool
		var ok bool
		for i, c := range compiled {
			book, ok = books.ParseFilename(f, c)
			if ok {
				parsed = true
				book.RegexpName = regexpNames[i]
				break
			}
		}
		if !parsed {
			log.Printf("Unable to parse %s", f)
			continue
		}
		title, tags := books.SplitTitleAndTags(book.Title)
		book.Title = title
		book.Tags = tags
		book.OriginalFilename = f
		fi, err := os.Stat(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing %s: %s\n", f, err)
			continue
		}
		book.FileSize = fi.Size()
		book.FileMtime = fi.ModTime()
		err = book.CalculateHash()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing %s: %s\n", f, err)
			continue
		}

		fmt.Printf("%+v\n", book)
		var tmpl *template.Template
		tmpl, err = template.New("filename").Funcs(template.FuncMap{"ToUpper": strings.ToUpper}).Parse(viper.GetString("output_template"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing output template: %s\n", err)
			continue
		}
		s, err := book.Filename(tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			continue
		}
		book.CurrentFilename = s
		err = library.ImportBook(book)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing book: %s\n", err)
		}
	}
}
