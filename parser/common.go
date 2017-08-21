package parser

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"time"
	"github.com/Azure/azure-sdk-for-go/storage"
)

type AzureEventRecord interface {
	IsInitialized() bool
	InitRecord()
	getSourceFileName() string
	GetTime() time.Time
	GetLogSourceName() string
	NewCEFEvent() CEFEvent
	GetCEFList(options GetCEFEventListOptions) ([]*CEFEvent, []error)
}

type AzureEventLog interface {
	GetRecords() []AzureEventRecord
}

type AzureLogFile interface {
	ShortName() string
	GetName() string
	GetAzureEventLog() AzureEventLog
	LoadBlob() error
	LoadBlobRange(blobRange storage.BlobRange) error
	getUnprocessedBlobRange() storage.BlobRange
	GetLastProcessed() time.Time
	GetLastProcessedRecord() time.Time
	GetLastProcessedTimeStamp() int64
	GetLastRecordCount() int
	GetLastModified() time.Time
	GetLastProcessedRange() storage.BlobRange
	SetLastProcessed(LastProcessed time.Time)
	SetLastProcessedTimeStamp(LastProcessedTimeStamp int64)
	SetLastRecordCount(LastRecordCount int)
	SetLastProcessedRecord(LastProcessedRecord time.Time)
	SetLastProcessedRange(LastProcessedRange storage.BlobRange)
	Logger() *log.Entry
	GetBlob() storage.Blob
	GetLogTime() time.Time
	GetNsgName() string
	GetEtag() string
}

func createProcessStatusFromLogfile(logfile AzureLogFile) LogFileProcessStatus {
	logFileProcessStatus := LogFileProcessStatus{}
	logFileProcessStatus.Name = logfile.GetName()
	logFileProcessStatus.Etag = logfile.GetEtag()
	logFileProcessStatus.LastModified = logfile.GetLastModified()
	logFileProcessStatus.LastProcessed = logfile.GetLastProcessed()
	logFileProcessStatus.LastProcessedRecord = logfile.GetLastProcessedRecord()
	logFileProcessStatus.LastProcessedTimeStamp = logfile.GetLastProcessedTimeStamp()
	logFileProcessStatus.LastRecordCount = logfile.GetLastRecordCount()
	logFileProcessStatus.LastProcessedRange = logfile.GetLastProcessedRange()
	logFileProcessStatus.LogTime = logfile.GetLogTime()
	logFileProcessStatus.NsgName = logfile.GetNsgName()
	return logFileProcessStatus
}

type LogFileProcessStatus struct {
	Name                   string            `json:"name"`
	Etag                   string            `json:"etag"`
	LastModified           time.Time         `json:"last_modified"`
	LastProcessed          time.Time         `json:"last_processed"`
	LastProcessedRecord    time.Time         `json:"last_processed_record"`
	LastProcessedTimeStamp int64             `json:"last_processed_timestamp"`
	LastRecordCount        int               `json:"last_count"`
	LastProcessedRange     storage.BlobRange `json:"last_processed_range"`
	LogTime                time.Time         `json:"log_time"`
	NsgName                string            `json:"nsg_name"`
}


// ProcessStatus is a simple map meant to store status for AzureLogFile
type ProcessStatus map[string]LogFileProcessStatus

type NsgParserClient interface {
	ProcessAzureLogFile(AzureLogFile, chan AzureLogFile) error
}

// Parses Blob.Name (Path) or Resource ID for NSG Name
func getLoggedResourceName(name string) (string, error) {
	nameTokens := LoggedResourceFileRegExp.FindStringSubmatch(name)

	if len(nameTokens) != 7 {
		log.Errorf("%d %s", len(nameTokens), name)
		return "", errResourceIdName
	}
	return nameTokens[1], nil
}

func formatMac(s string) string {
	var buffer bytes.Buffer
	var n_1 = 1
	var l_1 = len(s) - 1
	for i, rune := range s {
		buffer.WriteRune(rune)
		if i%2 == n_1 && i != l_1 {
			buffer.WriteRune(':')
		}
	}
	return buffer.String()
}
