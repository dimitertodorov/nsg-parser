package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"regexp"
	"time"
)

var (
	AppGwFileRegExp = regexp.MustCompile(`.*\/(.*)\/y=([0-9]{4})\/m=([0-9]{2})\/d=([0-9]{2})\/h=([0-9]{2})\/m=([0-9]{2}).*`)
)

// AzureAppGwLogFile represents individual .json Log files in azure
type AzureAppGwLogFile struct {
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
	AzureAppGwAccessLog    *AzureAppGwAccessLog `json:"-"`
	LoggedResourceName     string            `json:"nsg_name"`
}

func (logFile *AzureAppGwLogFile) GetLastProcessed() time.Time {
	return logFile.LastProcessed
}

func (logFile *AzureAppGwLogFile) SetLastProcessed(LastProcessed time.Time) {
	logFile.LastProcessed = LastProcessed
}

func (logFile *AzureAppGwLogFile) SetLastRecordCount(LastRecordCount int) {
	logFile.LastRecordCount = LastRecordCount
}

func (logFile *AzureAppGwLogFile) SetLastProcessedRecord(LastProcessedRecord time.Time) {
	logFile.LastProcessedRecord = LastProcessedRecord
}

func (logFile *AzureAppGwLogFile) SetLastProcessedRange(LastProcessedRange storage.BlobRange) {
	logFile.LastProcessedRange  = LastProcessedRange
}

func (logFile *AzureAppGwLogFile) SetLastProcessedTimeStamp(LastProcessedTimeStamp int64) {
	logFile.LastProcessedTimeStamp = LastProcessedTimeStamp
}

type AzureAppGwAccessLog struct {
	Records AzureAppGwEventRecords `json:"records"`
	azureEventRecords []AzureEventRecord
}

func (log *AzureAppGwAccessLog) GetRecords() []AzureEventRecord {
	if log.azureEventRecords == nil {
		log.azureEventRecords = make([]AzureEventRecord, len(log.Records))
		for i, v := range log.Records {
			log.azureEventRecords[i] = &v
		}
	}
	return log.azureEventRecords
}

func NewAzureAppGwLogFile(blob storage.Blob) (AzureLogFile, error) {
	nsgLogFile := AzureAppGwLogFile{}
	nsgLogFile.Blob = blob
	nsgLogFile.Name = blob.Name
	nsgLogFile.Etag = blob.Properties.Etag
	nsgLogFile.LastModified = time.Time(blob.Properties.LastModified)

	logTime, err := getAppGwLogTimeFromName(blob.Name)
	nsgLogFile.LogTime = logTime

	nsgName, err := getLoggedResourceName(blob.Name)
	nsgLogFile.LoggedResourceName = nsgName

	return &nsgLogFile, err
}

func NewAzureAppGwLogFileFromEventLog(eventLog *AzureAppGwAccessLog) (AzureAppGwLogFile, error) {
	nsgLogFile := AzureAppGwLogFile{}
	nsgLogFile.AzureAppGwAccessLog = eventLog
	if len(eventLog.GetRecords()) == 0 {
		return AzureAppGwLogFile{}, nil
	}
	record := eventLog.GetRecords()[0]
	if !record.IsInitialized() {
		record.InitRecord()
	}

	nsgLogFile.Name = record.getSourceFileName()
	nsgLogFile.LastModified = time.Time(record.GetTime())

	nsgLogFile.Logger().Info("**********The name", nsgLogFile.Name)
	logTime, err := getAppGwLogTimeFromName(nsgLogFile.Name)

	nsgLogFile.LogTime = logTime

	nsgLogFile.LoggedResourceName = record.GetLogSourceName()

	return nsgLogFile, err
}

func (logFile *AzureAppGwLogFile) ShortName() string {
	logTime := logFile.LogTime.Format("2006-01-02-15")
	return fmt.Sprintf("%s-%s", logFile.LoggedResourceName, logTime)
}

func (logFile *AzureAppGwLogFile) GetName() string {
	return logFile.Name
}

func (logFile *AzureAppGwLogFile) GetNsgName() string {
	return logFile.LoggedResourceName
}

func (logFile *AzureAppGwLogFile) GetEtag() string {
	return logFile.Etag
}

func (logFile *AzureAppGwLogFile) GetLogTime() time.Time {
	return logFile.LogTime
}

func (logFile *AzureAppGwLogFile) GetAzureEventLog() AzureEventLog {
	return logFile.AzureAppGwAccessLog
}

func (logFile *AzureAppGwLogFile) GetLastProcessedRecord() time.Time {
	return logFile.LastProcessedRecord
}

func (logFile *AzureAppGwLogFile) GetLastProcessedTimeStamp() int64 {
	return logFile.LastProcessedTimeStamp
}

func (logFile *AzureAppGwLogFile) GetLastRecordCount() int {
	return logFile.LastRecordCount
}

func (logFile *AzureAppGwLogFile) GetLastModified() time.Time {
	return logFile.LastModified
}

func (logFile *AzureAppGwLogFile) GetLastProcessedRange() storage.BlobRange {
	return logFile.LastProcessedRange
}

func (logFile *AzureAppGwLogFile) GetBlob() storage.Blob {
	return logFile.Blob
}

func (logFile *AzureAppGwLogFile) LoadBlob() error {
	blobRange := storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	return logFile.LoadBlobRange(blobRange)
}

// Primary function for loading the storage.Blob object into an NsgLog
// Range is a set of byte offsets for reading the contents.
func (logFile *AzureAppGwLogFile) LoadBlobRange(blobRange storage.BlobRange) error {
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
func (logFile *AzureAppGwLogFile) LoadAzureNsgEventRecords(payload []byte) error {
	err := json.Unmarshal(payload, &logFile.AzureAppGwAccessLog)
	return err
}

// Provides a github.com/sirupsen/logrus template .
func (logFile *AzureAppGwLogFile) Logger() *log.Entry {
	return log.WithFields(log.Fields{
		"ShortName":           logFile.ShortName(),
		"LastProcessedRecord": logFile.LastProcessedRecord,
		"LastModified":        logFile.LastModified,
		"Nsg":                 logFile.LoggedResourceName,
	})
}

func getAppGwLogTimeFromName(name string) (time.Time, error) {
	nameTokens := AppGwFileRegExp.FindStringSubmatch(name)

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


func (logFile *AzureAppGwLogFile) getUnprocessedBlobRange() storage.BlobRange {
	var blobRange storage.BlobRange
	if logFile.LastProcessedRange.End != 0 {
		blobRange = storage.BlobRange{Start: logFile.LastProcessedRange.End, End: uint64(logFile.Blob.Properties.ContentLength)}
	} else {
		blobRange = storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	}
	return blobRange
}
