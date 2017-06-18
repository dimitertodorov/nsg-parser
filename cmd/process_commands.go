package cmd

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/dimitertodorov/nsg-parser/parser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

var (
	accountName    string
	accountKey     string
	containerName  string
	nsgAzureClient parser.AzureClient
	syslogClient   parser.SyslogClient
	fileClient     parser.FileClient
	daemon         bool
	pollInterval   int
	prefix         string
	timeLayout     = "2006-01-02-15-04-05-GMT"
	serveHttp      bool
	serveBindIp    string
	servePort      int
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process NSG Files from Azure Blob Storage",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		log.Infof("Use a Subcommand - syslog or file")
	},
}

var syslogCmd = &cobra.Command{
	Use:   "syslog",
	Short: "Process NSG Files from Azure Blob Storage to Remote Syslog",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initSyslog()
		if serveHttp {
			go startHttpServer(cmd)
		}
		for {
			processSyslog(cmd)
			if !daemon {
				break
			}
			time.Sleep(time.Duration(pollInterval) * time.Second)
		}

	},
}

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Process NSG Files from Azure Blob Storage to Local File",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initFileClient()
		if serveHttp {
			go startHttpServer(cmd)
		}
		for {
			processFiles(cmd)
			if !daemon {
				break
			}
			time.Sleep(time.Duration(pollInterval) * time.Second)
		}

	},
}

func init() {
	processCmd.PersistentFlags().String("prefix", "", "Azure Blob Prefix. Optional")
	processCmd.PersistentFlags().String("storage_account_name", "", "Azure Account Name")
	processCmd.PersistentFlags().String("storage_account_key", "", "Azure Account Key")
	processCmd.PersistentFlags().String("container_name", "", "Azure Container Name")
	processCmd.PersistentFlags().String("begin_time", "2017-06-16-12", "Only process blobs for period after this time.")
	processCmd.PersistentFlags().BoolVarP(&daemon, "daemon", "d", false, "")
	processCmd.PersistentFlags().IntVar(&pollInterval, "poll_interval", 60, "Interval in Seconds to check Storage Account for Log updates.")

	processCmd.PersistentFlags().BoolVar(&serveHttp, "serve_http", false, "Serve an HTTP Endpoint with Status Details?")
	processCmd.PersistentFlags().StringVar(&serveBindIp, "serve_bind_ip", "127.0.0.1", "IP on which to serve. 0.0.0.0 for all.")
	processCmd.PersistentFlags().IntVar(&servePort, "serve_port", 3000, "Port on which to serve ")

	syslogCmd.PersistentFlags().String("syslog_protocol", "tcp", "tcp or udp")
	syslogCmd.PersistentFlags().String("syslog_host", "127.0.0.1", "Syslog Hostname or IP")
	syslogCmd.PersistentFlags().String("syslog_port", "5514", "Syslog Port")

	viper.BindPFlag("prefix", processCmd.PersistentFlags().Lookup("prefix"))
	viper.BindPFlag("storage_account_name", processCmd.PersistentFlags().Lookup("storage_account_name"))
	viper.BindPFlag("storage_account_key", processCmd.PersistentFlags().Lookup("storage_account_key"))
	viper.BindPFlag("container_name", processCmd.PersistentFlags().Lookup("container_name"))
	viper.BindPFlag("syslog_protocol", syslogCmd.PersistentFlags().Lookup("syslog_protocol"))
	viper.BindPFlag("syslog_host", syslogCmd.PersistentFlags().Lookup("syslog_host"))
	viper.BindPFlag("syslog_port", syslogCmd.PersistentFlags().Lookup("syslog_port"))

	RootCmd.AddCommand(processCmd)

	processCmd.AddCommand(syslogCmd)
	processCmd.AddCommand(fileCmd)

}

func initClient() {
	if devMode {
		accountName = storage.StorageEmulatorAccountName
		accountKey = storage.StorageEmulatorAccountKey
	} else {
		accountName = viper.GetString("storage_account_name")
		accountKey = viper.GetString("storage_account_key")
	}
	containerName = viper.GetString("container_name")

	prefix = viper.GetString("prefix")

	client, err := parser.NewAzureClient(accountName, accountKey, containerName, dataPath)
	if err != nil {
		log.Errorf("error creating storage client")
	}
	nsgAzureClient = client

}

func initSyslog() {
	slProtocol := viper.GetString("syslog_protocol")
	slHost := viper.GetString("syslog_host")
	slPort := viper.GetString("syslog_port")
	err := syslogClient.Initialize(slProtocol, slHost, slPort, &nsgAzureClient)
	if err != nil {
		log.Fatalf("error initializing syslog client %s", err)
	}
}

func initFileClient() {
	fileClient.Initialize(dataPath, &nsgAzureClient)
}

func processFiles(cmd *cobra.Command) {
	beginTime := cmd.Flags().Lookup("begin_time").Value.String()
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	err = nsgAzureClient.ProcessBlobsAfter(afterTime, fileClient)
	if err != nil {
		log.Error(err)
	}
}

func processSyslog(cmd *cobra.Command) {
	beginTime := cmd.Flags().Lookup("begin_time").Value.String()
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	err = nsgAzureClient.ProcessBlobsAfter(afterTime, syslogClient)
	if err != nil {
		log.Error(err)
	}

}

func startHttpServer(cmd *cobra.Command) {
	stdoutLog.WithFields(log.Fields{
		"Host": serveBindIp,
		"Port": servePort,
	}).Info("serving nsg-parser status  on HTTP")
	parser.ServeClient(&nsgAzureClient, serveBindIp, servePort)
}
