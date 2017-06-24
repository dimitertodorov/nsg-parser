package parser

import (
	"github.com/Azure/azure-sdk-for-go/storage"
	metrics "github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
	"fmt"
)

const (
	MAX_CONCURRENCY   = 1
	DestinationFile   = "file"
	DestinationSyslog = "syslog"
)

var (
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
	RegisteredJobs  map[string]*Job
}

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
	azureClient.Concurrency = MAX_CONCURRENCY
	azureClient.processMutex = &sync.Mutex{}
	azureClient.RegisteredJobs = make(map[string]*Job)

	log.Debugf("initialized AzureClient to %s", accountName)
	return azureClient, nil
}

func (client *FileClient) Initialize(dataPath string) error {
	client.DataPath = dataPath
	return nil
}

func (client *AzureClient) GetBlobsByPrefix(prefix string) ([]storage.Blob, error) {
	params := storage.ListBlobsParameters{
		Prefix: prefix,
	}
	list, err := client.container.ListBlobs(params)
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}

func (client *AzureClient) ProcessBlobsAfter(afterTime time.Time, parserClient NsgParserClient, jobName string) error {
	var job *Job
	jobOptions := &JobOptions{
		StartRecordTime: afterTime,
		DataPath:        client.DataPath,
	}

	job, _ = NewJob(jobOptions, make(ProcessStatus), client, parserClient)
	job.Name = jobName
	err := client.RegisterJob(job)
	if err != nil {
		return err
	}

	return client.RunJob(jobName)
}

func (client *AzureClient) RegisterJob(job *Job) error {
	client.RegisteredJobs[job.Name] = job
	return nil
}

func (client *AzureClient) RunJob(jobName string) error {
	job, ok := client.RegisteredJobs[jobName]
	if !ok {
		return fmt.Errorf("no existing job with %s", jobName)
	}else{
		job.LoadProcessStatus()
		job.LoadUnprocessedLogFiles()
		job.LoadTasks()
		job.Run()
		return nil
	}
}

func (logFile *AzureNsgLogFile) getUnprocessedBlobRange() storage.BlobRange {
	var blobRange storage.BlobRange
	if logFile.LastProcessedRange.End != 0 {
		blobRange = storage.BlobRange{Start: logFile.LastProcessedRange.End, End: uint64(logFile.Blob.Properties.ContentLength)}
	} else {
		blobRange = storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	}
	return blobRange
}
