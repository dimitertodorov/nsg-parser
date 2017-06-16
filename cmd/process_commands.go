package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
	"github.com/dimitertodorov/nsg-parser/parser"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

var (
	accountName    string
	accountKey     string
	containerName  string
	prefix         string
	dataPath       string
	blobCli        storage.BlobStorageClient
	nsgAzureClient parser.AzureClient
	syslogClient   parser.SyslogClient
	fileClient     parser.FileClient
	pollLogs       bool
	pollInterval   int

	timeLayout = "2006-01-02-15-04-05-GMT"
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
		processSyslog(cmd)
	},
}

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Process NSG Files from Azure Blob Storage to Local File",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initFileClient()
		processFiles(cmd)
	},
}

var scratchCmd = &cobra.Command{
	Use:   "scratch",
	Short: "Tester",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initSyslog()
		logs := []byte(`[{
    "time": 1497477570,
    "systemId": "",
    "category": "",
    "resourceId": "/SUBSCRIPTIONS/RGNAME/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG",
    "operationName": "",
    "rule": "Fake_UDP_RULE",
    "mac": "00:0D:3A:F3:38:54",
    "sourceIp": "10.193.60.4",
    "destinationIp": "10.44.55.66",
    "sourcePort": "14953",
    "destinationPort": "80",
    "protocol": "U",
    "trafficFlow": "O",
    "traffic": "D"
  },
  {
    "time": 1497477572,
    "systemId": "",
    "category": "",
    "resourceId": "/SUBSCRIPTIONS/RGNAME/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG",
    "operationName": "",
    "rule": "Fake_TCP_RULE",
    "mac": "00:0D:3A:F3:38:54",
    "sourceIp": "10.44.55.66",
    "destinationIp": "10.193.160.4",
    "sourcePort": "14954",
    "destinationPort": "80",
    "protocol": "T",
    "trafficFlow": "I",
    "traffic": "A"
  }]`)
		aLogs := []parser.NsgFlowLog{}
		_ = json.Unmarshal(logs, &aLogs)
		for _, flowLog := range aLogs {
			syslogClient.SendEvent(flowLog)
		}
	},
}

func init() {
	processCmd.PersistentFlags().StringP("data_path", "", "", "Where to Save the files")
	processCmd.PersistentFlags().StringP("prefix", "", "", "Prefix")
	processCmd.PersistentFlags().StringP("storage_account_name", "", "", "Account")
	processCmd.PersistentFlags().StringP("storage_account_key", "", "", "Key")
	processCmd.PersistentFlags().StringP("container_name", "", "", "Container Name")
	processCmd.PersistentFlags().StringP("begin_time", "", "2017-06-16-12", "Only Process Files after this time. 2017-01-01-01")
	processCmd.PersistentFlags().BoolVar(&pollLogs, "poll_logs", false, "Keep Process Running")
	processCmd.PersistentFlags().IntVar(&pollInterval, "poll_interval", 60, "Interval in Seconds to check Storage Account for Logs. ")

	syslogCmd.PersistentFlags().StringP("syslog_protocol", "", "tcp", "tcp or udp")
	syslogCmd.PersistentFlags().StringP("syslog_host", "", "127.0.0.1", "Syslog Hostname or IP")
	syslogCmd.PersistentFlags().StringP("syslog_port", "", "5514", "Syslog Port")

	viper.BindPFlag("data_path", processCmd.PersistentFlags().Lookup("data_path"))
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
	processCmd.AddCommand(scratchCmd)
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
	dataPath = viper.GetString("data_path")

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
