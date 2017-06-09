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
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	accountName   string
	accountKey    string
	containerName string
	prefix        string
	blobCli       storage.BlobStorageClient
	devBlobCli    storage.BlobStorageClient
)

var blobChan = make(chan storage.Blob, 2)
var quitChan = make(chan int)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process NSG Files from Azure Blob Storage",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		processSyslog()
	},
}

var syslogCmd = &cobra.Command{
	Use:   "syslog",
	Short: "Process NSG Files from Azure Blob Storage to Remote Syslog",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		processSyslog()
	},
}

var scratchCmd = &cobra.Command{
	Use:   "scratch",
	Short: "Tester",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		for {
			getFiles()
			log.Infof("Sleeping for 20 seconds.")
			break
			time.Sleep(20 * time.Second)

		}
	},
}

func init() {
	processCmd.PersistentFlags().StringP("path", "", "/tmp/azlog", "Where to Save the files")
	processCmd.PersistentFlags().StringP("prefix", "", "", "Prefix")
	processCmd.PersistentFlags().StringP("storage_account_name", "", "", "Account")
	processCmd.PersistentFlags().StringP("storage_account_key", "", "", "Key")
	processCmd.PersistentFlags().StringP("container_name", "", "", "Container Name")
	syslogCmd.PersistentFlags().StringP("syslog_protocol", "", "tcp", "tcp or udp")
	syslogCmd.PersistentFlags().StringP("syslog_host", "", "127.0.0.1", "Syslog Hostname or IP")
	syslogCmd.PersistentFlags().StringP("syslog_port", "", "5514", "Syslog Port")
	viper.BindPFlag("prefix", processCmd.PersistentFlags().Lookup("prefix"))
	viper.BindPFlag("storage_account_name", processCmd.PersistentFlags().Lookup("storage_account_name"))
	viper.BindPFlag("storage_account_key", processCmd.PersistentFlags().Lookup("storage_account_key"))
	viper.BindPFlag("container_name", processCmd.PersistentFlags().Lookup("container_name"))
	viper.BindPFlag("syslog_protocol", syslogCmd.PersistentFlags().Lookup("syslog_protocol"))
	viper.BindPFlag("syslog_host", syslogCmd.PersistentFlags().Lookup("syslog_host"))
	viper.BindPFlag("syslog_port", syslogCmd.PersistentFlags().Lookup("syslog_port"))
	RootCmd.AddCommand(processCmd)
	processCmd.AddCommand(syslogCmd)
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
	storageClient, err := storage.NewBasicClient(accountName, accountKey)
	if err != nil {
		log.Errorf("error creating storage client")
	}
	containerName = viper.GetString("container_name")
	blobCli = storageClient.GetBlobService()
	slProtocol := viper.GetString("syslog_protocol")
	slHost := viper.GetString("syslog_host")
	slPort := viper.GetString("syslog_port")
	nsgsyslog.InitClient(slProtocol, slHost, slPort)
}

func getFiles() {
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	after := "2017-06-09-19"
	timeLayout := "2006-01-02-15-04-05-GMT"
	afterTime, err := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", after))
	processedFiles := make(map[string]*model.NsgLogFile)
	oldProcessStatus, err := LoadProcessStatus()
	if err != nil {
		log.Errorf("%s", err)
		os.Exit(2)
	}
	log.Info(oldProcessStatus)
	for _, blob := range matchingBlobs {
		beat, _ := model.NewNsgLogFile(&blob)
		if beat.LogTime.After(afterTime) {
			lastBeat, ok := oldProcessStatus[beat.Blob.Name]
			if ok {
				if beat.LastModified.After(lastBeat.LastModified) {
					log.Infof("Modified Blob - %s", beat.Blob.Name)
				} else {
					log.Infof("Not Modified Blob - %s", lastBeat.Name)
					processedFiles[lastBeat.Name] = lastBeat
					//continue
				}
			}
			err = beat.LoadBlob()
			if err != nil {
				log.Errorf("%s", err)
			}
			err = beat.SaveToPath("/Users/todorovd/azlog/")
			if err != nil {
				log.Errorf("%s", err)
			}
			beat.LastProcessed = time.Now()
			beat.LastCount = len(beat.NsgLog.Records)
			log.Infof("Time: %s", beat.NsgLog.Records[1].Time.Format(time.RFC822Z))
			log.Errorf("%s", beat.Blob.Name)
			processedFiles[beat.Blob.Name] = &beat
		} else {
			log.Infof("after date ignoring %v", beat.Name)
		}
	}
	outJson, err := json.Marshal(processedFiles)
	if err != nil {
		log.Errorf("error marshalling to disk")
	}
	path := "/Users/todorovd/azlog/process-status.json"
	err = ioutil.WriteFile(path, outJson, 0666)
}

func LoadProcessStatus() (map[string]*model.NsgLogFile, error) {
	path := "/Users/todorovd/azlog/process-status.json"
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

func oldprocessSyslog() {
	fmt.Println("Processing Syslog")
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-quitChan:
				log.Debugf("Closing")
				close(blobChan)
				return
			default:
				for blob := range blobChan {
					log.Debugf("Processing Blob %v", blob.Name)
					processSysBlob(blob)
				}
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for _, blob := range matchingBlobs {
			log.Infof("Blob %v", blob.Name)
			blobChan <- blob
		}
		quitChan <- 1
		return
	}()
	wg.Wait()
}

func processSysBlob(b storage.Blob) error {
	log.Infof("Processing %v", b.Name)
	nsgLog, err := getNsgLogs(b)
	if err != nil {
		log.Error(err)
		return err
	} else {
		alogs, _ := nsgLog.ConvertToNsgFlowLog()
		for _, nsgEvent := range *alogs {
			nsgsyslog.SendEvent(nsgEvent)
		}
	}
	return nil
}

func processBlob(b storage.Blob, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Infof("Processing %v", b.Name)
	nsgLog, err := getNsgLogs(b)
	if err != nil {
		log.Error(err)
	} else {
		alogs := []model.NsgFlowLog{}
		for _, record := range nsgLog.Records {
			for _, flow := range record.Properties.Flows {
				for _, subFlow := range flow.Flows {
					for _, flowTuple := range subFlow.FlowTuples {
						alog := model.NsgFlowLog{}
						tuples := strings.Split(flowTuple, ",")
						epochTime, _ := strconv.ParseInt(tuples[0], 10, 64)
						alog.ResourceID = record.ResourceID
						alog.Timestamp = epochTime
						alog.SourceIp = tuples[1]
						alog.DestinationIp = tuples[2]
						alog.SourcePort = tuples[3]
						alog.DestinationPort = tuples[4]
						alog.Protocol = tuples[5]
						alog.TrafficFlow = tuples[6]
						alog.Traffic = tuples[7]
						alog.Rule = flow.Rule
						alog.Mac = subFlow.Mac
						alogs = append(alogs, alog)
					}
				}
			}
		}
		outJson, _ := json.Marshal(alogs)
		logName, _ := getAsFileName(b)
		err = ioutil.WriteFile(logName, outJson, 0666)
		log.Infof("Wrote File: %v . Events: %v", logName, len(alogs))
		if err != nil {
			log.Errorf("write file failed: %v", err)
			os.Exit(1)
		}
	}
}

func getAsFileName(b storage.Blob) (string, error) {
	bm := model.NsgFileRegExp.FindStringSubmatch(b.Name)
	if len(bm) == 7 {
		fileName := fmt.Sprintf("nsgLog-%s-%s%s%s%s%s", bm[1], bm[2], bm[3], bm[4], bm[5], bm[6])

		return fileName, nil
	} else {
		return "", fmt.Errorf("Error Parsing Blob.Name")
	}
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
	log.Infof("Got Blobs %v", list.Blobs)
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}
