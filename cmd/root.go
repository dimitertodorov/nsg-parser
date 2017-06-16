package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var cfgFile string
var devMode bool
var debug bool

var RootCmd = &cobra.Command{
	Use:   "nsg-parser",
	Short: "GO NSG Toolkit",
	Long:  `A fast NSG tool`,
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initViper()
		initLogging(cmd)
	},
}

func init() {


	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/nsg-parser.json)")
	RootCmd.PersistentFlags().BoolVar(&devMode, "dev_mode", false, "DEV MODE: Use Storage Emulator? \n Must be reachable at http://127.0.0.1:10000")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "DEBUG? Turn on Debug logging with this.")

}

func initLogging(cmd *cobra.Command) {
	log.SetOutput(os.Stdout)

	if debug{
		log.SetLevel(log.DebugLevel)
	}else{
		log.SetLevel(log.InfoLevel)
	}
}

func initViper() {
	viper.AddConfigPath("/etc/nsg-parser/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.nsg-parser") // call multiple times to add many search paths
	viper.AddConfigPath(".")                 // optionally look for config in the working directory
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Printf("")
		}

		// Search config in home directory with name "gomi" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("nsg-parser")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		log.Debug(fmt.Sprintf("Using config file: %v", viper.ConfigFileUsed()))
	} else {
		log.Panic(fmt.Sprintf("Error Loading Config File - %v - Err: %v", viper.ConfigFileUsed(), err))
	}
}
