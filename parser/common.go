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
}

type NsgParserClient interface {
	ProcessNsgLogFile(AzureLogFile, chan AzureLogFile) error
}

// Parses Blob.Name (Path) or Resource ID for NSG Name
func getNsgName(name string) (string, error) {
	nameTokens := NsgFileRegExp.FindStringSubmatch(name)

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
