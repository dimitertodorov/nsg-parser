package parser

import (
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
	syslog "github.com/RackSec/srslog"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"time"
	"text/template"
	"bytes"
	"github.com/dimitertodorov/nsg-parser/pool"
)

const (
	DestinationFile   = iota
	DestinationSyslog
)

var (
	syslogFormat = "nsgflow:{{.Timestamp}},{{.Rule}},{{.Mac}},{{.SourceIp}},{{.SourcePort}},{{.DestinationIp}},{{.DestinationPort}},{{.Protocol}},{{.TrafficFlow}},{{.Traffic}}"
)

type AzureClient struct {
	storageClient   storage.Client
	blobClient      storage.BlobStorageClient
	container       *storage.Container
	Prefix          string
	ProcessStatus   ProcessStatus
	DataPath        string
	DestinationType int
	Concurrency     int
}

type SyslogClient struct {
	writer      *syslog.Writer
	template    template.Template
	initialized bool
}

type FileClient struct {
	DataPath string
}

type ProcessStatus map[string]*NsgLogFile

func NewAzureClient(accountName, accountKey, containerName, dataPath string) (AzureClient, error) {
	azureClient := AzureClient{}

	storageClient, err := storage.NewBasicClient(accountName, accountKey)
	if err != nil {
		return azureClient, err
	}
	azureClient.storageClient = storageClient
	azureClient.blobClient = storageClient.GetBlobService()
	azureClient.container = azureClient.blobClient.GetContainerReference(containerName)
	azureClient.DataPath = dataPath
	azureClient.Concurrency = 1
	log.Debugf("Initialized AzureClient to %s", accountName)
	return azureClient, nil
}

func (client *FileClient) Initialize(dataPath string, azureClient *AzureClient) error {
	client.DataPath = dataPath

	azureClient.DestinationType = DestinationFile
	azureClient.LoadProcessStatus()

	return nil
}

func (client *SyslogClient) Initialize(protocol, host, port string, azureClient *AzureClient) (error) {
	syslogWriter, err := syslog.Dial(protocol, fmt.Sprintf("%s:%s", host, port),
		syslog.LOG_ERR, "nsg-parser")
	if err != nil {
		log.Fatal(err)
		return err
	}

	syslogWriter.SetFormatter(syslog.RFC5424Formatter)

	logTemplate, err := template.New("nsgFlowTemplate").Parse(syslogFormat)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	client.template = *logTemplate
	client.writer = syslogWriter
	client.initialized = true

	azureClient.DestinationType = DestinationSyslog
	azureClient.LoadProcessStatus()

	return nil
}

func (client *SyslogClient) SendEvent(flowLog NsgFlowLog) error {
	var message bytes.Buffer
	if !client.initialized {
		return fmt.Errorf("cannot SendEvent() to uninitialized syslog")
	}
	err := client.template.Execute(&message, flowLog)
	if err != nil {
		return fmt.Errorf("event_format_error %s", err)
	}
	fmt.Fprintf(client.writer, "%s", message.String())
	return nil
}

func (client *AzureClient) GetBlobsByPrefix(prefix string) ([]storage.Blob, error) {
	params := storage.ListBlobsParameters{}
	list, err := client.container.ListBlobs(params)
	log.Debugf("GetBlobsByPrefix %v", len(list.Blobs))
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}

func (client *AzureClient) LoadProcessStatus() error {
	path := fmt.Sprintf("%s/%s", client.DataPath, client.ProcessStatusFileName())
	var processMap ProcessStatus
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file error: %v\n", err)
	}
	err = json.Unmarshal(file, &processMap)
	if err != nil {
		return fmt.Errorf("file error: %v\n", err)
	}
	client.ProcessStatus = processMap
	log.Debugf("GOT SOME EVENTS %s", path)
	return nil
}

func (client *AzureClient) LoadUnprocessedBlobs(afterTime time.Time) (*[]NsgLogFile, ProcessStatus, error) {
	var nsgLogFiles []NsgLogFile

	processStatus := ProcessStatus{}
	matchingBlobs, err := client.GetBlobsByPrefix("")
	if err != nil {
		return &nsgLogFiles, processStatus, fmt.Errorf("Error Loading Blob List - Error %v", err)
	}

	for _, blob := range matchingBlobs {
		logFile, _ := NewNsgLogFile(blob)
		if logFile.LogTime.After(afterTime) {
			lastProcessedFile, ok := client.ProcessStatus[logFile.Blob.Name]
			if ok {
				if logFile.LastModified.After(lastProcessedFile.LastModified) {
					logFile.LastProcessedTimeStamp = lastProcessedFile.LastProcessedTimeStamp
					logFile.LastProcessedRecord = lastProcessedFile.LastProcessedRecord
					logFile.Logger().Info("processing modified blob")
					nsgLogFiles = append(nsgLogFiles, logFile)
				} else {
					lastProcessedFile.Logger().Info("skipping unmodified blob")
					processStatus[lastProcessedFile.Name] = lastProcessedFile
					continue
				}
			} else {
				logFile.Logger().Info("processing new blob")
				nsgLogFiles = append(nsgLogFiles, logFile)
			}
		} else {
			logFile.Logger().Debug("before begin_date")
		}
	}
	return &nsgLogFiles, processStatus, nil
}

func (client *AzureClient) ProcessBlobsAfter(afterTime time.Time, parserClient NsgParserClient) (error) {
	var tasks = []*pool.Task{}
	var numErrors int
	resultsChan := make(chan NsgLogFile)
	done := make(chan bool)

	logFiles, processedFiles, err := client.LoadUnprocessedBlobs(afterTime)
	if err != nil {
		return err
	}
	for _, logFile := range *logFiles {
		taskFile := logFile
		blobTask := pool.NewTask(func() error {
			taskFile.Logger().WithField("ClientType",fmt.Sprintf("%T", parserClient)).Debug("processing started")
			return parserClient.ProcessNsgLogFile(&taskFile, resultsChan)
		})
		tasks = append(tasks, blobTask)
	}

	p := pool.NewPool(tasks, client.Concurrency)
	go func() {
		for {
			processedFile, more := <-resultsChan
			if more {
				processedFiles[processedFile.Blob.Name] = &processedFile
				processedFile.Logger().WithField("ClientType",fmt.Sprintf("%T", parserClient)).Debug("processing completed")
			} else {
				log.Infof("finished all files")
				done <- true
				return
			}
		}
	}()
	p.Run()
	close(resultsChan)
	<-done

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
	client.ProcessStatus = processedFiles
	err = client.SaveProcessStatus()
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (client *AzureClient) ProcessStatusFileName() string {
	if client.DestinationType == DestinationSyslog {
		return "nsg-parser-status-syslog.json"
	} else if client.DestinationType == DestinationFile {
		return "nsg-parser-status-file.json"
	} else {
		return "nsg-parser-status"
	}

}

func (client *AzureClient) SaveProcessStatus() error {
	outJson, err := json.Marshal(client.ProcessStatus)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/%s", client.DataPath, client.ProcessStatusFileName())
	err = ioutil.WriteFile(path, outJson, 0666)
	return err
}

func (client SyslogClient) ProcessNsgLogFile(logFile *NsgLogFile, resultsChan chan NsgLogFile) error {
	err := logFile.LoadBlob()
	if err != nil {
		log.Error(err)
		return err
	} else {
		filteredLogs, _ := logFile.NsgLog.GetFlowLogsAfter(logFile.LastProcessedRecord)
		logCount := len(filteredLogs)
		endTimeStamp := filteredLogs[logCount-1].Timestamp
		logFile.LastProcessedTimeStamp = endTimeStamp
		for _, nsgEvent := range filteredLogs {
			client.SendEvent(nsgEvent)
		}
	}
	logFile.LastProcessed = time.Now()
	logFile.LastRecordCount = len(logFile.NsgLog.Records)
	logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
	resultsChan <- *logFile
	return nil
}

func (client FileClient) ProcessNsgLogFile(logFile *NsgLogFile, resultsChan chan NsgLogFile) error {
	var fileName string
	err := logFile.LoadBlob()
	if err != nil {
		log.Error(err)
		return err
	} else {
		filteredLogs, err := logFile.NsgLog.GetFlowLogsAfter(logFile.LastProcessedRecord)
		if err != nil {
			return err
		}
		logCount := len(filteredLogs)
		startTimeStamp := filteredLogs[0].Timestamp
		endTimeStamp := filteredLogs[logCount-1].Timestamp

		bm := NsgFileRegExp.FindStringSubmatch(logFile.Blob.Name)
		if len(bm) == 7 {
			fileName = fmt.Sprintf("nsgLog-%s-%s%s%s%s%s", bm[1], bm[2], bm[3], bm[4], bm[5], bm[6])
		} else {
			return fmt.Errorf("Error Parsing Blob.Name")
		}

		if logCount == 0 {
			log.Debugf("nothing needs doing - no new logs for %s", logFile.Blob.Name)
			return nil
		}


		fileName = fmt.Sprintf("%s-%d-%d.json", fileName, startTimeStamp, endTimeStamp)
		outJson, err := json.Marshal(filteredLogs)
		if err != nil {
			return fmt.Errorf("error marshalling to json")
		}

		path := fmt.Sprintf("%s/%s", client.DataPath, fileName)
		err = ioutil.WriteFile(path, outJson, 0666)

		logFile.LastProcessed = time.Now()
		logFile.LastRecordCount = len(logFile.NsgLog.Records)
		logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
		logFile.LastProcessedTimeStamp = endTimeStamp
		resultsChan <- *logFile
		return nil
	}
	return nil
}
