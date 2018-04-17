// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"fmt"
	"os"

	"github.com/foomo/htpasswd"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List authenticated users for the web server",
	Run: func(cmd *cobra.Command, args []string) {
		listUsers(cmd, args)
	},
}

func init() {
	authCmd.AddCommand(listCmd)
}

func listUsers(cmd *cobra.Command, args []string) {
	if _, err := os.Stat(htpasswdFile); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Authentication is disabled. Add a user to enable.")
		os.Exit(1)
	}

	passwords, err := htpasswd.ParseHtpasswdFile(htpasswdFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot list users: %s\n", err)
		os.Exit(1)
	}
	for user, _ := range passwords {
		fmt.Println(user)
	}
}
