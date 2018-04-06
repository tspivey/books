// Copyright © 2018 Author

package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tspivey/books"
)

var overrideExistingLibrary bool = false

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the library",
	Long:  `Initialize a new empty library`,
	Run: func(cmd *cobra.Command, args []string) {
		dbFile := libraryFile
		if _, err := os.Stat(dbFile); err == nil {
			if !overrideExistingLibrary {
				fmt.Fprintf(os.Stderr, "A library already exists in %s. Use -f to forcefully override the existing library, or choose another configuration directory.\n", dbFile)
				os.Exit(1)
			}
			fmt.Println("Warning: overriding existing library")
			if err := os.Remove(dbFile); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot remove existing library: %s\n", err)
				os.Exit(1)
			}
		}
		if err := books.CreateLibrary(dbFile); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create library: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	initCmd.Flags().BoolVarP(&overrideExistingLibrary, "forceOverride", "f", false, "Override a library if one already exists.")
}
