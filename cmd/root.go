package cmd

import (
	"fmt"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"time"
	"path/filepath"
)

var (
	cfgFile       string
	devMode       bool
	debug         bool
	dataPath      string
	logNameFormat = `nsg-parser-%Y%m%d%H%M.log`
	stdoutLog     *log.Logger
	httpProxy	string
)

var RootCmd = &cobra.Command{
	Use:   "nsg-parser",
	Short: "GO NSG Toolkit",
	Long:  `A fast NSG tool`,
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initViper()
		initDataPath()
		initLogging()
		initProxy()
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/nsg-parser.json)")
	RootCmd.PersistentFlags().BoolVar(&devMode, "dev_mode", false, "DEV MODE: Use Storage Emulator? \n Must be reachable at http://127.0.0.1:10000")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "DEBUG? Turn on Debug logging with this.")

	RootCmd.PersistentFlags().String("data_path", "", "Full path to store status and processed files.")
	RootCmd.PersistentFlags().String("http_proxy", "", "Equivalent to exporting http_proxy and https_proxy in environment. Useful for service config.")

	viper.BindPFlag("data_path", RootCmd.PersistentFlags().Lookup("data_path"))
	viper.BindPFlag("http_proxy", RootCmd.PersistentFlags().Lookup("http_proxy"))
}

func initDataPath() {
	dataPath = viper.GetString("data_path")
}

func initProxy() {
	if proxy := viper.GetString("http_proxy"); proxy != "" {
		log.WithField("proxy", proxy).Info("using proxy")
		os.Setenv("HTTP_PROXY", proxy)
		os.Setenv("HTTPS_PROXY", proxy)
	}
}

func initLogging() {
	stdoutLog = log.New()
	stdoutLog.Out = os.Stdout

	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{})

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	logf, err := rotatelogs.New(
		logPath(),
		rotatelogs.WithMaxAge(24*time.Hour),
		rotatelogs.WithRotationTime(time.Hour),
	)
	if err != nil {
		log.Fatalf("failed to create rotatelogs: %s", err)
	}

	log.SetOutput(logf)

	logFields := log.Fields{
		"path":  logPath(),
		"logLevel": log.GetLevel().String(),
	}
	log.WithFields(logFields).Info("started logging")

	//CurrentFileName() doesn't get return anything until first write.
	logFields["current_file"] = logf.CurrentFileName()

	stdoutLog.WithFields(logFields).Info("started logging")
}

func logPath() string {
	return filepath.Join(dataPath, logNameFormat)
}

func initViper() {
	viper.AddConfigPath("/etc/nsg-parser/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.nsg-parser") // call multiple times to add many search paths
	viper.AddConfigPath(".")                 // optionally look for config in the working directory
	if cfgFile, err := cfgFilePath(); err == nil{
		log.Error(cfgFile)
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			log.WithField("config_file", viper.ConfigFileUsed()).
				Fatal("unable to load provided config file. exiting")
		}
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
		log.WithField("config_file", viper.ConfigFileUsed()).
			Info("loaded config file")
	} else {
		log.WithField("config_file", viper.ConfigFileUsed()).
			Error("error loading config file")
	}
}

func cfgFilePath() (string, error) {
	if cfgFile == "" {
		return "", fmt.Errorf("no cfgFile provided")
	}
	return filepath.Abs(cfgFile)
}
