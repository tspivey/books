// Copyright © 2018 Author

package commands

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var overrideExistingLibrary bool = false
var initialSchema = `create table books (
id integer primary key,
created_on timestamp not null default (datetime()),
updated_on timestamp not null default (datetime()),
author text not null,
series text,
title text not null,
extension text not null,
tags text,
original_filename text not null,
filename text not null,
file_size integer not null,
file_mtime timestamp not null,
hash text not null unique,
regexp_name text not null,
template_override text,
source text
);
create virtual table books_fts using fts4 (author, series, title, extension, tags,  filename, source);
`

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the library",
	Long:  `Initialize a new empty library`,
	Run: func(cmd *cobra.Command, args []string) {
		dbFile := viper.GetString("db")
		if _, err := os.Stat(dbFile); err == nil {
			if !overrideExistingLibrary {
				fmt.Fprintf(os.Stderr, "A library already exists in %s. Use -f to forcefully override the existing library, or update db in the config file.\n", dbFile)
				os.Exit(1)
			}
			fmt.Println("Warning: overriding existing library")
			if err := os.Remove(dbFile); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot remove existing library: %s\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("Initializing library in %s\n", dbFile)
		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		_, err = db.Exec(initialSchema)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Done.")
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
