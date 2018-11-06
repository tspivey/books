// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime/pprof"
	"strings"
	"text/template"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tspivey/books"
)

var cfgDir string
var libraryFile string
var booksRoot string
var cpuProfile string
var htpasswdFile string
var funcMap = template.FuncMap{
	"inc": func(i int) int {
		return i + 1
	},
	"joinNaturally": joinNaturally,
	"ToUpper":       strings.ToUpper,
	"join":          strings.Join,
	"escape":        books.Escape,
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "books",
	Short: "Books is a library for all of your books",
	Long: `Books manages all of your books.

To create a new library, type books init.
Modify your config file in $HOME/.config/books/config.toml, setting up your matching regular expressions.
Then run books import path/to/books.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", "", "config directory (default is $HOME/.config/books)")
	rootCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "CPU profile filename")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfgDir != "" {
		// Use config dir from the flag.
		viper.AddConfigPath(cfgDir)
	} else {
		// Search for config in $HOME/.config/books
		cfgDir = path.Join(home, ".config", "books")
		viper.AddConfigPath(cfgDir)
	}
	libraryFile = path.Join(cfgDir, "books.db")
	htpasswdFile = path.Join(cfgDir, "htpasswd")
	viper.SetConfigName("config")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config file: %s\n", err)
		os.Exit(1)
	}

	viper.SetDefault("root", path.Join(home, "books"))
	booksRoot = viper.GetString("root")
}

// CPUProfile wraps a cobra command for CPU profiling.
func CPUProfile(f func(cmd *cobra.Command, args []string)) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		var fp *os.File
		if cpuProfile != "" {
			fp, err := os.Create(cpuProfile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.StartCPUProfile(fp)
		}
		f(cmd, args)
		if cpuProfile != "" {
			pprof.StopCPUProfile()
			fp.Close()
		}
	}
}

// joinNaturally joins a slice of strings separated by a comma and space,
// putting the conjunction before the last item.
// If there are only two items, they will be separated by the conjunction (surrounded by spaces), with no comma.
// Examples:
// first item
// first item and second item
// first item, second item, and third item
func joinNaturally(conjunction string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 {
		return fmt.Sprintf("%s %s %s", items[0], conjunction, items[1])
	}
	return fmt.Sprintf("%s, %s %s", strings.Join(items[:len(items)-1], ", "), conjunction, items[len(items)-1])
}
