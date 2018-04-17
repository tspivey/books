// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"bytes"
	"fmt"
	"os"

	"github.com/foomo/htpasswd"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <username>",
	Short: "Add an authenticated user to the web server",
	Long: `Add an authenticated user to the web server.

You will be prompted to enter a new password. Passwords are encrypted with Bcrypt.`,
	Run: func(cmd *cobra.Command, args []string) {
		addUser(cmd, args)
	},
}

func init() {
	authCmd.AddCommand(addCmd)
}

func addUser(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No username specified")
		cmd.Usage()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Password: ")
	pass, err := gopass.GetPasswdMasked()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get password: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Confirm password: ")
	confirmedPass, err := gopass.GetPasswdMasked()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get confirmed password: %s\n", err)
		os.Exit(1)
	}
	if !bytes.Equal(pass, confirmedPass) {
		fmt.Fprintln(os.Stderr, "Passwords do not match")
		os.Exit(1)
	}

	if err := htpasswd.SetPassword(htpasswdFile, args[0], string(pass), htpasswd.HashBCrypt); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot set password for user %s: %s\n", args[0], err)
		os.Exit(1)
	}
	fmt.Println("Password updated")
}
