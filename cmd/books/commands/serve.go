// Copyright © 2018 Tyler Spivey <tspivey@pcdesk.net> and Niko Carpenter <nikoacarpenter@gmail.com>
//
// This source code is governed by the MIT license, which can be found in the LICENSE file.

// Copyright © 2018 Author

package commands

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/tspivey/books"
	"github.com/tspivey/books/server"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the library from a web server",
	Long:  `Bring up a web server and serve the library.`,
	Run:   runServer,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringP("bind", "b", "127.0.0.1:8000", "Bind the server to host:port. Leave host empty to bind to all interfaces.")
	serveCmd.Flags().IntP("conversion-workers", "c", 4, "Number of conversion workers to run")
	viper.BindPFlag("server.bind", serveCmd.Flags().Lookup("bind"))
	viper.BindPFlag("server.conversion_workers", serveCmd.Flags().Lookup("conversion-workers"))
	viper.SetDefault("server.read_timeout", 5)
	viper.SetDefault("server.write_timeout", 0)
	viper.SetDefault("server.idle_timeout", 120)
	viper.SetDefault("server.items_per_page", 20)
}

func runServer(cmd *cobra.Command, args []string) {
	cacheDir := path.Join(cfgDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %s\n", err)
		os.Exit(1)
	}
	templatesDir := path.Join(cfgDir, "templates")
	lib, err := books.OpenLibrary(libraryFile, booksRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening library: %s\n", err)
		os.Exit(1)
	}

	numConversionWorkers := viper.GetInt("server.conversion_workers")
	converter := server.NewCalibreBookConverter(booksRoot, cacheDir, numConversionWorkers)
	log.Printf("Starting %d workers for converting books", numConversionWorkers)

	hsrv := &http.Server{
		Addr:         viper.GetString("server.bind"),
		ReadTimeout:  viper.GetDuration("server.read_timeout") * time.Second,
		WriteTimeout: viper.GetDuration("server.write_timeout") * time.Second,
		IdleTimeout:  viper.GetDuration("server.idle_timeout") * time.Second,
	}

	outputTmplSrc := viper.GetString("output_template")
	outputTmpl, err := template.New("filename").Funcs(template.FuncMap{"ToUpper": strings.ToUpper, "join": strings.Join, "escape": books.Escape}).Parse(outputTmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse output template: %s\n\n%s\n", err, outputTmplSrc)
		os.Exit(1)
	}

	cfg := &server.Config{
		Lib:            lib,
		TemplatesDir:   templatesDir,
		Converter:      converter,
		ItemsPerPage:   viper.GetInt("server.items_per_page"),
		Hsrv:           hsrv,
		HtpasswdFile:   htpasswdFile,
		BooksRoot:      booksRoot,
		OutputTemplate: outputTmpl,
	}
	srv := server.New(cfg)
	log.Printf("Listening on %s", hsrv.Addr)
	log.Printf("Read timeout: %d, write timeout: %d, idle timeout: %d seconds", hsrv.ReadTimeout/time.Second, hsrv.WriteTimeout/time.Second, hsrv.IdleTimeout/time.Second)
	log.Fatal(srv.Start())
}
