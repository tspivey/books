// Copyright © 2018 Author

package commands

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime/pprof"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var cpuProfile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "books",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.books.yaml)")
	rootCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "CPU profile filename")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".books" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".books")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config file: %s\n", err)
		os.Exit(1)
	}
	viper.SetDefault("root", path.Join(home, "books"))
	viper.SetDefault("db", path.Join(home, "books.db"))
}

func CpuProfile(f func(cmd *cobra.Command, args []string)) func(cmd *cobra.Command, args []string) {
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
