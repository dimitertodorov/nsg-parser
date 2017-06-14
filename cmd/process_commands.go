package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/Sirupsen/logrus"
	"github.com/dimitertodorov/nsg-parser/model"
	"github.com/dimitertodorov/nsg-parser/nsgsyslog"
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
	devBlobCli    storage.BlobStorageClient
	timeLayout    = "2006-01-02-15-04-05-GMT"
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
		processSyslog()
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
		for {
			log.Infof("Sleeping for 20 seconds.")
			break
			time.Sleep(20 * time.Second)

		}
	},
}

func init() {
	processCmd.PersistentFlags().StringP("data_path", "", "", "Where to Save the files")
	processCmd.PersistentFlags().StringP("prefix", "", "", "Prefix")
	processCmd.PersistentFlags().StringP("storage_account_name", "", "", "Account")
	processCmd.PersistentFlags().StringP("storage_account_key", "", "", "Key")
	processCmd.PersistentFlags().StringP("container_name", "", "", "Container Name")

	syslogCmd.PersistentFlags().StringP("syslog_protocol", "", "tcp", "tcp or udp")
	syslogCmd.PersistentFlags().StringP("syslog_host", "", "127.0.0.1", "Syslog Hostname or IP")
	syslogCmd.PersistentFlags().StringP("syslog_port", "", "5514", "Syslog Port")

	fileCmd.PersistentFlags().StringP("begin_time", "", "2017-06-14-01", "Only Process Files after this time. 2017-01-01-01")

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

	prefix = viper.GetString("prefix")

	dataPath = viper.GetString("data_path")

	storageClient, err := storage.NewBasicClient(accountName, accountKey)
	if err != nil {
		log.Errorf("error creating storage client")
	}
	containerName = viper.GetString("container_name")
	blobCli = storageClient.GetBlobService()

}

func initSyslog() {
	slProtocol := viper.GetString("syslog_protocol")
	slHost := viper.GetString("syslog_host")
	slPort := viper.GetString("syslog_port")
	nsgsyslog.InitClient(slProtocol, slHost, slPort)
}

func processFiles(cmd *cobra.Command) {
	var lastTimeStamp int64
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	//after := "2017-06-14-19"
	beginTime := cmd.Flags().Lookup("begin_time").Value.String()

	lastTimeStamp = 0
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
	processedFiles := make(map[string]*model.NsgLogFile)
	oldProcessStatus, err := LoadProcessStatus()
	if err != nil {
		log.Warnf("process-status.json does not exist. Processing All Files %s", err)
	}
	for _, blob := range matchingBlobs {
		logFile, _ := model.NewNsgLogFile(&blob)
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
			logFile.LastCount = len(logFile.NsgLog.Records)
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

func LoadProcessStatus() (map[string]*model.NsgLogFile, error) {
	path := fmt.Sprintf("%s/process-status.json", dataPath)
	var processMap map[string]*model.NsgLogFile
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

func processSyslog() {
	var tasks = []*pool.Task{}
	fmt.Println("Processing Syslog")
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	log.Debugf("Processing %s blobs", len(matchingBlobs))
	for _, blob := range matchingBlobs {
		thisBlob := blob
		blobTask := pool.NewTask(func() error { return processSysBlob(thisBlob) })
		tasks = append(tasks, blobTask)
	}
	p := pool.NewPool(tasks, 4)
	p.Run()
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
}

func processSysBlob(b storage.Blob) error {
	log.Infof("Processing %v", b.Name)
	nsgLog, err := getNsgLogs(b)
	if err != nil {
		log.Error(err)
		return err
	} else {
		alogs, _ := nsgLog.ConvertToNsgFlowLogs()
		for _, nsgEvent := range alogs {
			nsgsyslog.SendEvent(nsgEvent)
		}
	}
	return nil
}

func getNsgLogs(b storage.Blob) (model.NsgLog, error) {
	readCloser, err := b.Get(nil)
	nsgLog := model.NsgLog{}
	if err != nil {
		return nsgLog, fmt.Errorf("get blob failed: %v", err)
	}
	defer readCloser.Close()
	bytesRead, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return nsgLog, fmt.Errorf("read body failed: %v", err)
	}
	err = json.Unmarshal(bytesRead, &nsgLog)
	if err != nil {

		err = b.Delete(nil)
		log.Errorf("Delete Result %v", err)
		return nsgLog, fmt.Errorf("json parse body failed: %v - %v", err, b.Name)
	}
	return nsgLog, nil
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
