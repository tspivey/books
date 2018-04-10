// Copyright © 2018 author

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
