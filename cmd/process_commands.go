package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
	"github.com/dimitertodorov/nsg-parser/parser"
	"github.com/dimitertodorov/nsg-parser/pool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"time"
)

var (
	accountName   string
	accountKey    string
	containerName string
	prefix        string
	dataPath          string
	blobCli       storage.BlobStorageClient
	timeLayout    = "2006-01-02-15-04-05-GMT"
	parserCli	parser.AzureClient
	syslogClient parser.SyslogClient
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
		_ = json.Unmarshal(logs,&aLogs)
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
	parserCli=client


}

func initSyslog() {
	slProtocol := viper.GetString("syslog_protocol")
	slHost := viper.GetString("syslog_host")
	slPort := viper.GetString("syslog_port")
	err := syslogClient.Initialize(slProtocol, slHost, slPort)
	if err != nil {
		log.Fatalf("error initializing syslog client %s", err)
	}
}

func processFiles(cmd *cobra.Command) {
	var lastTimeStamp int64
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	beginTime := cmd.Flags().Lookup("begin_time").Value.String()

	lastTimeStamp = 0
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	processedFiles := make(map[string]*parser.NsgLogFile)
	oldProcessStatus, err := LoadProcessStatus()
	if err != nil {
		log.Warnf("process-status.json does not exist. Processing All Files %s", err)
	}
	for _, blob := range matchingBlobs {
		logFile, _ := parser.NewNsgLogFile(blob)
		if logFile.LogTime.After(afterTime) {
			lastBeat, ok := oldProcessStatus[logFile.Blob.Name]
			if ok {

				if logFile.LastModified.After(lastBeat.LastModified) {
					lastTimeStamp = lastBeat.LastProcessedTimeStamp
					log.Debugf("processing modified blob - %s-%s", lastBeat.NsgName, lastBeat.ShortName())
				} else {
					log.Infof("blob already processed - %s-%s", lastBeat.NsgName, lastBeat.ShortName())
					processedFiles[lastBeat.Name] = lastBeat
					continue
				}
			}
			err = logFile.LoadBlob()
			if err != nil {
				log.Errorf("error processing %s - %s", err, blob.Name)
			}

			filePath, err := logFile.ConvertToPath(dataPath, lastTimeStamp); if err != nil {
				log.Errorf("%s", err)
			}

			logFile.LastProcessed = time.Now()
			logFile.LastRecordCount = len(logFile.NsgLog.Records)
			log.Infof("processed %s", filePath)
			processedFiles[logFile.Blob.Name] = &logFile
		} else {
			log.Infof("before  begin_date ignoring %v", logFile.ShortName())
		}
	}
	outJson, err := json.Marshal(processedFiles)
	if err != nil {
		log.Errorf("error marshalling to disk")
	}
	path := fmt.Sprintf("%s/process-status.json", dataPath)
	err = ioutil.WriteFile(path, outJson, 0666)
}

func LoadProcessStatus() (map[string]*parser.NsgLogFile, error) {
	path := fmt.Sprintf("%s/process-status.json", dataPath)
	var processMap map[string]*parser.NsgLogFile
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return processMap, fmt.Errorf("File error: %v\n", err)
	}
	err = json.Unmarshal(file, &processMap)
	if err != nil {
		return processMap, fmt.Errorf("File error: %v\n", err)
	}
	return processMap, nil

}

func processSyslog(cmd *cobra.Command) {
	var tasks = []*pool.Task{}

	beginTime := cmd.Flags().Lookup("begin_time").Value.String()
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))

	resultsChan := make(chan parser.NsgLogFile)
	done := make(chan bool)
	logFiles, processedFiles, err := parserCli.LoadUnprocessedBlobs(afterTime)
	for _, logFile := range *logFiles {
		taskFile := logFile
		blobTask := pool.NewTask(func() error { return processSysBlob(taskFile, resultsChan) })
		tasks = append(tasks, blobTask)
	}
	p := pool.NewPool(tasks, 4)
	go func() {
		for {
			processedFile, more := <-resultsChan
			if more {
				processedFiles[processedFile.Blob.Name] = &processedFile
				log.Debugf("processed file %s", processedFile.Blob.Name)
			} else {
				log.Infof("finished all files")
				done <- true
				return
			}
		}
	}()
	p.Run()
	close(resultsChan)
	<- done
	var numErrors int
	for _, task := range p.Tasks {
		if task.Err != nil {
			log.Error(task.Err)
			numErrors++
		}
		if numErrors >= 10 {
			log.Error("Too many errors.")
			break
		}
	}

	outJson, err := json.Marshal(processedFiles)
	if err != nil {
		log.Errorf("error marshalling to disk")
	}else{
		log.Debugf("%s", string(outJson[:]))
	}
	path := fmt.Sprintf("%s/process-status.json", dataPath)

	err = ioutil.WriteFile(path, outJson, 0666)
	if err != nil{
		log.Fatalf("Error Saving Process Status", err)
	}
	parserCli.LoadProcessStatus()
}

func processSysBlob(logFile parser.NsgLogFile, resultsChan chan parser.NsgLogFile) error {
	log.Infof("syslog processing %v", logFile.ShortName())
	err := logFile.LoadBlob()
	if err != nil {
		log.Error(err)
		return err
	} else {
		//alogs, _ := logFile.NsgLog.ConvertToNsgFlowLogs()
		filteredLogs, _ := logFile.NsgLog.GetFlowLogsAfter(logFile.LastProcessedRecord)
		logCount := len(filteredLogs)
		endTimeStamp := filteredLogs[logCount-1].Timestamp
		logFile.LastProcessedTimeStamp = endTimeStamp
		log.Debugf("Got %d events for %d - %d", logCount, logFile.LastProcessedTimeStamp, len(filteredLogs))
		for _, nsgEvent := range filteredLogs {
			syslogClient.SendEvent(nsgEvent)
		}
	}
	logFile.LastProcessed = time.Now()
	logFile.LastRecordCount = len(logFile.NsgLog.Records)
	logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
	resultsChan <- logFile
	return nil
}

func getBlobList(cnt *storage.Container) ([]storage.Blob, error) {
	params := storage.ListBlobsParameters{}
	params.Prefix = prefix
	list, err := cnt.ListBlobs(params)
	log.Debugf("Got Blobs %v", list.Blobs)
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}
