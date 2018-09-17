// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the version of Books.
var Version = "unset"

// Copyright is the copyright including authors of Books.
var Copyright = "Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Books",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Books version %s\n%s\n", Version, Copyright)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
