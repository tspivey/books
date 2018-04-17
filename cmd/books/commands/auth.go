// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

package commands

import (
	"github.com/spf13/cobra"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage basic authentication for the web server",
	Long: `Add, remove, and list authenticated users.

Authorized users are stored in htpasswd format, in lib_root/htpasswd.
Delete this file to disable authentication.`,
}

func init() {
	rootCmd.AddCommand(authCmd)
}
