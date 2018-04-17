// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"fmt"

	"github.com/tspivey/books"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Books",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Books version %s\n", books.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
