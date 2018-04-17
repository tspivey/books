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

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove <username>",
	Short: "Remove an authenticated user from the web server",
	Long: `Remove an authenticated user from the web server.
When the last user is removed, authentication will still be enabled. This means that nobody can access the server until a user is added.
This is to prevent someone from accidentally making a library public by removing a user.`,
	Run: func(cmd *cobra.Command, args []string) {
		removeUser(cmd, args)
	},
}

func init() {
	authCmd.AddCommand(removeCmd)
}

func removeUser(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No username specified")
		cmd.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(htpasswdFile); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Authentication is disabled. Add a user to enable.")
		os.Exit(1)
	}

	if err := htpasswd.RemoveUser(htpasswdFile, args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot remove user %s: %s\n", args[0], err)
		os.Exit(1)
	}
	fmt.Println("Removed user")
}
