// Copyright © 2018 Author

package cmd

import (
	"log"
	"regexp"

	"books/internal"
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
	Run: importFunc,
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
	cobra.MarkFlagRequired(importCmd.Flags(), "regexp")
}

func importFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		log.Fatal("Filename to import is required.")
	}
	res, err := cmd.Flags().GetStringSlice("regexp")
	if err != nil {
		log.Fatal(err)
	}
	var compiled []*regexp.Regexp
	for _, v := range res {
		reString := viper.GetString("regexps." + v)
		if reString == "" {
			log.Fatalf("Regexp %s not found in config", v)
		}
		c, err := regexp.Compile(reString)
		if err != nil {
			log.Fatalf("Cannot compile regular expression %s: %s", v, err)
		}
		compiled = append(compiled, c)
	}
	for _, f := range args {
		var book internal.Book
		var ok bool
		var parsed bool
		for _, c := range compiled {
			book, ok = internal.ParseFilename(f, c)
			if ok {
				parsed = true
				break
			}
		}
		if !parsed {
			log.Printf("Unable to parse %s", f)
			continue
		}
		fmt.Printf("%+v\n", book)
	}
}
