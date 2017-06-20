package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	syslog "github.com/RackSec/srslog"
	"github.com/dimitertodorov/nsg-parser/pool"
	metrics "github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"sync"
	"text/template"
	"time"
)

const (
	DestinationFile   = "file"
	DestinationSyslog = "syslog"
)

var (
	syslogFormat       = "nsgflow:{{.Timestamp}},{{.Rule}},{{.Mac}},{{.SourceIp}},{{.SourcePort}},{{.DestinationIp}},{{.DestinationPort}},{{.Protocol}},{{.TrafficFlow}},{{.Traffic}}"
	syslogFormatter    = syslog.RFC5424Formatter
	processedFlowCount = metrics.GetOrRegisterCounter("processed_events", nil)
)

type AzureClient struct {
	storageClient   storage.Client
	blobClient      storage.BlobStorageClient
	container       *storage.Container
	Prefix          string
	ProcessStatus   ProcessStatus
	DataPath        string
	DestinationType string
	Concurrency     int
	processMutex    *sync.Mutex
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
	azureClient.processMutex = &sync.Mutex{}

	log.Debugf("initialized AzureClient to %s", accountName)
	return azureClient, nil
}

func (client *FileClient) Initialize(dataPath string, azureClient *AzureClient) error {
	client.DataPath = dataPath

	azureClient.DestinationType = DestinationFile

	if err := azureClient.LoadProcessStatus(); err != nil {
		return err
	}
	return nil
}

func (client *SyslogClient) Initialize(protocol, host, port string, azureClient *AzureClient) error {
	syslogWriter, err := syslog.Dial(protocol, fmt.Sprintf("%s:%s", host, port),
		syslog.LOG_ERR, "nsg-parser")
	if err != nil {
		log.Fatal(err)
		return err
	}

	syslogWriter.SetFormatter(syslogFormatter)

	logTemplate, err := template.New("nsgFlowTemplate").Parse(syslogFormat)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	client.template = *logTemplate
	client.writer = syslogWriter
	client.initialized = true

	azureClient.DestinationType = DestinationSyslog
	if err = azureClient.LoadProcessStatus(); err != nil {
		return err
	}
	return nil
}

func (client *SyslogClient) SendEvent(flowLog NsgFlowLog) error {
	var message bytes.Buffer
	if !client.initialized {
		return fmt.Errorf("uninitialized syslog client")
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
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}

func (client *AzureClient) LoadProcessStatus() error {
	processStatus, err := ReadProcessStatus(client.DataPath, client.ProcessStatusFileName())
	if err != nil {
		return err
	}
	client.ProcessStatus = processStatus
	return nil
}

func ReadProcessStatus(path, fileName string) (ProcessStatus, error) {
	var processStatus ProcessStatus
	filePath := filepath.Join(path, fileName)

	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return processStatus, nil
	}

	err = json.Unmarshal(file, &processStatus)
	if err != nil {
		return processStatus, fmt.Errorf("unmarshal error: %v\n", err)
	}
	return processStatus, nil
}

func (client *AzureClient) LoadUnprocessedBlobs(afterTime time.Time) (*[]NsgLogFile, ProcessStatus, error) {
	var nsgLogFiles []NsgLogFile

	processStatus := ProcessStatus{}

	matchingBlobs, err := client.GetBlobsByPrefix(client.Prefix)
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
					logFile.LastProcessedRange = lastProcessedFile.LastProcessedRange
					logFile.Logger().Info("processing modified blob")
					nsgLogFiles = append(nsgLogFiles, logFile)
				} else {
					lastProcessedFile.Logger().Debug("skipping unmodified blob")
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

//This is the primary function for processing NSG Flow Blobs.
func (client *AzureClient) ProcessBlobsAfter(afterTime time.Time, parserClient NsgParserClient) error {
	var tasks = []*pool.Task{}
	var numErrors int

	client.processMutex.Lock()
	defer client.processMutex.Unlock()

	resultsChan := make(chan NsgLogFile)
	done := make(chan bool)

	logFiles, processedFiles, err := client.LoadUnprocessedBlobs(afterTime)
	if err != nil {
		return err
	}

	for _, logFile := range *logFiles {
		taskFile := logFile
		blobTask := pool.NewTask(func() error {
			taskFile.Logger().WithField("type", fmt.Sprintf("%T", parserClient)).Debug("processing started")
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
				processedFile.Logger().WithField("type", fmt.Sprintf("%T", parserClient)).Debug("processing completed")
			} else {
				log.WithField("type", fmt.Sprintf("%T", parserClient)).
					Info("processing completed")
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
			log.Error("too many errors.")
			break
		}
	}

	client.ProcessStatus = processedFiles

	err = client.SaveProcessStatus()
	if err != nil {
		log.Error(err)
		return err
	}
	client.LoadProcessStatus()
	return nil
}

func (client *AzureClient) ProcessStatusFileName() string {
	return fmt.Sprintf("nsg-parser-status-%s.json", client.DestinationType)
}

func (client *AzureClient) SaveProcessStatus() error {
	outJson, err := json.Marshal(client.ProcessStatus)
	if err != nil {
		return err
	}
	path := filepath.Join(client.DataPath, client.ProcessStatusFileName())
	err = ioutil.WriteFile(path, outJson, 0666)
	return err
}

func (client SyslogClient) ProcessNsgLogFile(logFile *NsgLogFile, resultsChan chan NsgLogFile) error {
	blobRange := logFile.getUnprocessedBlobRange()
	err := logFile.LoadBlobRange(blobRange)
	if err != nil {
		log.Error(err)
		return err
	}

	filteredLogs, err := logFile.NsgLog.GetFlowLogsAfter(logFile.LastProcessedRecord)
	if err != nil {
		return err
	}

	logCount := len(filteredLogs)
	endTimeStamp := filteredLogs[logCount-1].Timestamp
	logFile.LastProcessedTimeStamp = endTimeStamp
	for _, nsgEvent := range filteredLogs {
		client.SendEvent(nsgEvent)
	}

	logFile.LastProcessed = time.Now()
	logFile.LastRecordCount = len(logFile.NsgLog.Records)
	logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
	logFile.LastProcessedRange = blobRange

	processedFlowCount.Inc(int64(logCount))

	resultsChan <- *logFile
	return nil
}

func (client FileClient) ProcessNsgLogFile(logFile *NsgLogFile, resultsChan chan NsgLogFile) error {
	var fileName string
	blobRange := logFile.getUnprocessedBlobRange()
	err := logFile.LoadBlobRange(blobRange)
	if err != nil {
		log.Error(err)
		return err
	}

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
		return fmt.Errorf("error in Blob.Name, expected 7 tokens. Got %d. Name: %s", len(bm), logFile.Blob.Name)
	}

	if logCount == 0 {
		log.Debugf("no new logs for %s", logFile.Blob.Name)
		return nil
	}

	fileName = fmt.Sprintf("%s-%d-%d.json", fileName, startTimeStamp, endTimeStamp)
	outJson, err := json.Marshal(filteredLogs)
	if err != nil {
		return fmt.Errorf("error marshalling to json %s", err)
	}
	path := filepath.Join(client.DataPath, fileName)
	err = ioutil.WriteFile(path, outJson, 0666)

	logFile.LastProcessed = time.Now()
	logFile.LastRecordCount = len(logFile.NsgLog.Records)
	logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
	logFile.LastProcessedTimeStamp = endTimeStamp
	logFile.LastProcessedRange = blobRange

	processedFlowCount.Inc(int64(logCount))

	resultsChan <- *logFile
	return nil
}

func (logFile *NsgLogFile) getUnprocessedBlobRange() storage.BlobRange {
	var blobRange storage.BlobRange
	if logFile.LastProcessedRange.End != 0 {
		blobRange = storage.BlobRange{Start: logFile.LastProcessedRange.End, End: uint64(logFile.Blob.Properties.ContentLength)}
	} else {
		blobRange = storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	}
	return blobRange
}
