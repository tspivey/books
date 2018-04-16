// Copyright © 2018 Author

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reader string
var justPrintFilename bool

// readCmd represents the read command
var readCmd = &cobra.Command{
	Use:   "read FILE_ID",
	Short: "Read a book",
	Long:  `Read a book given a file ID in the configured reader.`,
	Run:   CpuProfile(readRun),
}

func readRun(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No file ID specified.")
		os.Exit(1)
	}

	fileId, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "File ID must be a number.")
		os.Exit(1)
	}

	lib, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open library: %s\n", err)
		os.Exit(1)
	}

	files, err := lib.GetFilesById([]int64{int64(fileId)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while getting file by id: %s\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No file found")
		os.Exit(1)
	} else if len(files) > 1 {
		fmt.Fprintln(os.Stderr, "More than one file found; exiting")
		os.Exit(1)
	}
	filename := path.Join(booksRoot, files[0].CurrentFilename)

	if justPrintFilename {
		fmt.Println(filename)
		return
	}

	if reader == "" {
		reader = viper.GetString("readers." + strings.ToLower(files[0].Extension))
		if reader == "" {
			reader = viper.GetString("readers.DEFAULT")
			fmt.Println("Using default reader")
		} else {
			fmt.Printf("Using reader for format: %s\n", strings.ToLower(files[0].Extension))
		}
	}
	if reader == "" {
		fmt.Fprintln(os.Stderr, "No reader has been configured.")
		os.Exit(1)
	}

	if err := exec.Command(reader, filename).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read book: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(readCmd)

	readCmd.Flags().StringVarP(&reader, "reader", "R", "", "Override reader from config file")
	readCmd.Flags().BoolVarP(&justPrintFilename, "just-print-filename", "p", false, "Just print the filename and exit")
}
