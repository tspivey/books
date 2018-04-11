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
		if _, err := os.Stat(libraryFile); err == nil {
			if !overrideExistingLibrary {
				fmt.Fprintf(os.Stderr, "A library already exists in %s. Use -f to forcefully override the existing library, or choose another configuration directory.\n", libraryFile)
				os.Exit(1)
			}
			fmt.Println("Warning: overriding existing library")
			if err := os.Remove(libraryFile); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot remove existing library: %s\n", err)
				os.Exit(1)
			}
		}

		if err := books.CreateLibrary(libraryFile); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create library: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&overrideExistingLibrary, "forceOverride", "f", false, "Override a library if one already exists.")
}
