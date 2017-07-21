package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"
)

var (
	NsgFileRegExp = regexp.MustCompile(`.*\/(.*)\/y=([0-9]{4})\/m=([0-9]{2})\/d=([0-9]{2})\/h=([0-9]{2})\/m=([0-9]{2}).*`)
)

// AzureNsgLogFile represents individual .json Log files in azure
type AzureNsgLogFile struct {
	Name                   string            `json:"name"`
	Etag                   string            `json:"etag"`
	LastModified           time.Time         `json:"last_modified"`
	LastProcessed          time.Time         `json:"last_processed"`
	LastProcessedRecord    time.Time         `json:"last_processed_record"`
	LastProcessedTimeStamp int64             `json:"last_processed_timestamp"`
	LastRecordCount        int               `json:"last_count"`
	LastProcessedRange     storage.BlobRange `json:"last_processed_range"`
	LogTime                time.Time         `json:"log_time"`
	Blob                   storage.Blob      `json:"-"`
	AzureNsgEventLog       *AzureNsgEventLog `json:"-"`
	NsgName                string            `json:"nsg_name"`
}

func (logFile *AzureNsgLogFile) SetLastProcessed(LastProcessed time.Time) {
	logFile.LastProcessed = LastProcessed
}

func (logFile *AzureNsgLogFile) SetLastRecordCount(LastRecordCount int) {
	logFile.LastRecordCount = LastRecordCount
}

func (logFile *AzureNsgLogFile) SetLastProcessedRecord(LastProcessedRecord time.Time) {
	logFile.LastProcessedRecord = LastProcessedRecord
}

func (logFile *AzureNsgLogFile) SetLastProcessedRange(LastProcessedRange storage.BlobRange) {
	logFile.LastProcessedRange  = LastProcessedRange
}

func (logFile *AzureNsgLogFile) SetLastProcessedTimeStamp(LastProcessedTimeStamp int64) {
	logFile.LastProcessedTimeStamp = LastProcessedTimeStamp
}

// ProcessStatus is a simple map meant to store status for AzureNsgLogFile
type ProcessStatus map[string]AzureLogFile

type AzureNsgEventLog struct {
	Records AzureNsgEventRecords `json:"records"`
	azureEventRecords []AzureEventRecord
}

func (log *AzureNsgEventLog) GetRecords() []AzureEventRecord {
	if log.azureEventRecords == nil {
		log.azureEventRecords = make([]AzureEventRecord, len(log.Records))
		for i, v := range log.Records {
			log.azureEventRecords[i] = &v
		}
	}
	return log.azureEventRecords
}

func NewAzureNsgLogFile(blob storage.Blob) (AzureNsgLogFile, error) {
	nsgLogFile := AzureNsgLogFile{}
	nsgLogFile.Blob = blob
	nsgLogFile.Name = blob.Name
	nsgLogFile.Etag = blob.Properties.Etag
	nsgLogFile.LastModified = time.Time(blob.Properties.LastModified)

	logTime, err := getLogTimeFromName(blob.Name)
	nsgLogFile.LogTime = logTime

	nsgName, err := getNsgName(blob.Name)
	nsgLogFile.NsgName = nsgName

	return nsgLogFile, err
}

func NewAzureNsgLogFileFromEventLog(eventLog *AzureNsgEventLog) (AzureNsgLogFile, error) {
	nsgLogFile := AzureNsgLogFile{}
	nsgLogFile.AzureNsgEventLog = eventLog
	if len(eventLog.GetRecords()) == 0 {
		return AzureNsgLogFile{}, nil
	}
	record := eventLog.GetRecords()[0]
	if !record.IsInitialized() {
		record.InitRecord()
	}

	nsgLogFile.Name = record.getSourceFileName()
	nsgLogFile.LastModified = time.Time(record.GetTime())

	logTime, err := getLogTimeFromName(nsgLogFile.Name)
	nsgLogFile.LogTime = logTime

	nsgLogFile.NsgName = record.GetLogSourceName()

	return nsgLogFile, err
}

func (logFile *AzureNsgLogFile) ShortName() string {
	logTime := logFile.LogTime.Format("2006-01-02-15")
	return fmt.Sprintf("%s-%s", logFile.NsgName, logTime)
}

func (logFile *AzureNsgLogFile) GetName() string {
	return logFile.Name
}

func (logFile *AzureNsgLogFile) GetAzureEventLog() AzureEventLog {
	return logFile.AzureNsgEventLog
}

func (logFile *AzureNsgLogFile) GetLastProcessedRecord() time.Time {
	return logFile.LastProcessedRecord
}

func (logFile *AzureNsgLogFile) GetLastProcessedTimeStamp() int64 {
	return logFile.LastProcessedTimeStamp
}

func (logFile *AzureNsgLogFile) GetLastRecordCount() int {
	return logFile.LastRecordCount
}

func (logFile *AzureNsgLogFile) GetLastModified() time.Time {
	return logFile.LastModified
}

func (logFile *AzureNsgLogFile) GetLastProcessedRange() storage.BlobRange {
	return logFile.LastProcessedRange
}

func (logFile *AzureNsgLogFile) GetBlob() storage.Blob {
	return logFile.Blob
}

func (logFile *AzureNsgLogFile) LoadBlob() error {
	blobRange := storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	return logFile.LoadBlobRange(blobRange)
}

// Primary function for loading the storage.Blob object into an NsgLog
// Range is a set of byte offsets for reading the contents.
func (logFile *AzureNsgLogFile) LoadBlobRange(blobRange storage.BlobRange) error {
	bOptions := storage.GetBlobRangeOptions{
		Range: &blobRange,
	}
	readCloser, err := logFile.Blob.GetRange(&bOptions)
	if err != nil {
		logFile.Logger().Fatalf("get blob range failed: %v", err)
	}
	defer readCloser.Close()

	bytesRead, err := ioutil.ReadAll(readCloser)
	firstRecord := bytes.Index(bytesRead, []byte(`"time"`))
	if firstRecord == -1 {
		return fmt.Errorf("failed to find \"time\" in JSON payload")
	}
	structuredJson := []byte(`{"records": [{ `)
	structuredJson = append(structuredJson, bytesRead[firstRecord:]...)

	return logFile.LoadAzureNsgEventRecords(structuredJson)
}

// Ability to load JSON files from sources other than an Azure Blob.
func (logFile *AzureNsgLogFile) LoadAzureNsgEventRecords(payload []byte) error {
	err := json.Unmarshal(payload, &logFile.AzureNsgEventLog)
	return err
}

// Provides a github.com/sirupsen/logrus template .
func (logFile *AzureNsgLogFile) Logger() *log.Entry {
	return log.WithFields(log.Fields{
		"ShortName":           logFile.ShortName(),
		"LastProcessedRecord": logFile.LastProcessedRecord,
		"LastModified":        logFile.LastModified,
		"Nsg":                 logFile.NsgName,
	})
}

func ReadProcessStatus(path, fileName string) (ProcessStatus, error) {
	processStatus := make(ProcessStatus)
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

func getLogTimeFromName(name string) (time.Time, error) {
	nameTokens := NsgFileRegExp.FindStringSubmatch(name)

	if len(nameTokens) != 7 {
		return time.Time{}, errResourceIdName
	}

	timeLayout := "01/02 15:04:05 GMT 2006"
	year := nameTokens[2]
	month := nameTokens[3]
	day := nameTokens[4]
	hour := nameTokens[5]
	minute := nameTokens[6]

	timeString := fmt.Sprintf("%s/%s %s:%s:00 GMT %s", month, day, hour, minute, year)

	return time.Parse(timeLayout, timeString)
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
