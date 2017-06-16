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
)

var (
	syslogFormat = "nsgflow:{{.Timestamp}},{{.Rule}},{{.Mac}},{{.SourceIp}},{{.SourcePort}},{{.DestinationIp}},{{.DestinationPort}},{{.Protocol}},{{.TrafficFlow}},{{.Traffic}}"
)

type AzureClient struct {
	storageClient storage.Client
	blobClient    storage.BlobStorageClient
	container     *storage.Container
	Prefix        string
	ProcessStatus ProcessStatus
	DataPath      string
}

type SyslogClient struct {
	writer *syslog.Writer
	template template.Template
	initialized bool
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
	azureClient.LoadProcessStatus()
	log.Debugf("Initialized AzureClient to %s", accountName)
	return azureClient, nil
}

func (client *SyslogClient) Initialize(protocol, host, port string) (error) {
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
	path := fmt.Sprintf("%s/process-status.json", client.DataPath)
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
					log.Debugf("processing modified blob - %s-%s", lastProcessedFile.NsgName, lastProcessedFile.ShortName())
					logFile.LastProcessedTimeStamp = lastProcessedFile.LastProcessedTimeStamp
					logFile.LastProcessedRecord = lastProcessedFile.LastProcessedRecord
					nsgLogFiles = append(nsgLogFiles, logFile)
				} else {
					log.Debugf("blob already processed - %s-%s", lastProcessedFile.NsgName, lastProcessedFile.ShortName())
					processStatus[lastProcessedFile.Name] = lastProcessedFile
					continue
				}
			} else {
				nsgLogFiles = append(nsgLogFiles, logFile)
			}
		} else {
			log.Debugf("before  begin_date ignoring %v", logFile.ShortName())
		}
	}
	return &nsgLogFiles, processStatus, nil
}
