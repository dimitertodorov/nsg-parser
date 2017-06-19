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
	serveBind    string
	destinationType	string
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process NSG Files from Azure Blob Storage",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		var processFunc func()
		switch destinationType = viper.GetString("destination"); destinationType {
		case parser.DestinationFile:
			initFileClient()
			processFunc = processFiles
		case parser.DestinationSyslog:
			initSyslog()
			processFunc = processSyslog
		default:
			log.Fatalf("type must be one of file or syslog")
		}
		if serveHttp {
			go startHttpServer()
		}
		for {
			processFunc()
			if !daemon {
				break
			}
			time.Sleep(time.Duration(pollInterval) * time.Second)
		}
	},
}

func init() {
	processCmd.PersistentFlags().String("prefix", "", "Azure Blob Prefix. Optional")
	processCmd.PersistentFlags().String("destination", "file", "file or syslog")

	processCmd.PersistentFlags().String("storage_account_name", "", "Azure Account Name")
	processCmd.PersistentFlags().String("storage_account_key", "", "Azure Account Key")
	processCmd.PersistentFlags().String("container_name", "", "Azure Container Name")
	processCmd.PersistentFlags().String("begin_time", "2017-06-16-12", "Only process blobs for period after this time.")
	processCmd.PersistentFlags().BoolVarP(&daemon, "daemon", "d", false, "")
	processCmd.PersistentFlags().Int( "poll_interval", 60, "Interval in Seconds to check Storage Account for Log updates.")

	processCmd.PersistentFlags().Bool("serve_http",  false, "Serve an HTTP Endpoint with Status Details?")
	processCmd.PersistentFlags().String("serve_http_bind", "127.0.0.1:9889", "IP:PORT on which to serve. 0.0.0.0 for all.")

	processCmd.PersistentFlags().String("syslog_protocol", "tcp", "Syslog Protocol. tcp or udp")
	processCmd.PersistentFlags().String("syslog_host", "127.0.0.1", "Syslog Hostname or IP")
	processCmd.PersistentFlags().String("syslog_port", "5514", "Syslog Port")

	viper.BindPFlag("prefix", processCmd.PersistentFlags().Lookup("prefix"))
	viper.BindPFlag("destination", processCmd.PersistentFlags().Lookup("destination"))
	viper.BindPFlag("begin_time", processCmd.PersistentFlags().Lookup("begin_time"))

	viper.BindPFlag("storage_account_name", processCmd.PersistentFlags().Lookup("storage_account_name"))
	viper.BindPFlag("storage_account_key", processCmd.PersistentFlags().Lookup("storage_account_key"))
	viper.BindPFlag("container_name", processCmd.PersistentFlags().Lookup("container_name"))
	viper.BindPFlag("serve_http",processCmd.PersistentFlags().Lookup("serve_http"))
	viper.BindPFlag("serve_http_bind",processCmd.PersistentFlags().Lookup("serve_http_bind"))

	viper.BindPFlag("syslog_protocol", processCmd.PersistentFlags().Lookup("syslog_protocol"))
	viper.BindPFlag("syslog_host", processCmd.PersistentFlags().Lookup("syslog_host"))
	viper.BindPFlag("syslog_port", processCmd.PersistentFlags().Lookup("syslog_port"))

	RootCmd.AddCommand(processCmd)
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

	serveHttp = viper.GetBool("serve_http")
	serveBind = viper.GetString("serve_http_bind")

	pollInterval = viper.GetInt("poll_interval")

	destinationType = viper.GetString("destination")

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

func processFiles() {
	beginTime := viper.GetString("begin_time")
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	err = nsgAzureClient.ProcessBlobsAfter(afterTime, fileClient)
	if err != nil {
		log.Error(err)
	}
}

func processSyslog() {
	beginTime := viper.GetString("begin_time")
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	err = nsgAzureClient.ProcessBlobsAfter(afterTime, syslogClient)
	if err != nil {
		log.Error(err)
	}

}

func startHttpServer() {
	log.WithFields(log.Fields{
		"Host": viper.GetString("serve_http_bind"),
	}).Info("serving nsg-parser status  on HTTP")
	parser.ServeClient(&nsgAzureClient, serveBind)
}
