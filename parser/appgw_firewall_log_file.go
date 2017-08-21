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
	AppGwFirewallFileRegExp = regexp.MustCompile(`.*\/(.*)\/y=([0-9]{4})\/m=([0-9]{2})\/d=([0-9]{2})\/h=([0-9]{2})\/m=([0-9]{2}).*`)
)

// AzureAppGwFirewallLogFile represents individual .json Log files in azure
type AzureAppGwFirewallLogFile struct {
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
	AzureAppGwFirewallAccessLog    *AzureAppGwFirewallAccessLog `json:"-"`
	LoggedResourceName     string            `json:"nsg_name"`
}

func (logFile *AzureAppGwFirewallLogFile) GetLastProcessed() time.Time {
	return logFile.LastProcessed
}

func (logFile *AzureAppGwFirewallLogFile) SetLastProcessed(LastProcessed time.Time) {
	logFile.LastProcessed = LastProcessed
}

func (logFile *AzureAppGwFirewallLogFile) SetLastRecordCount(LastRecordCount int) {
	logFile.LastRecordCount = LastRecordCount
}

func (logFile *AzureAppGwFirewallLogFile) SetLastProcessedRecord(LastProcessedRecord time.Time) {
	logFile.LastProcessedRecord = LastProcessedRecord
}

func (logFile *AzureAppGwFirewallLogFile) SetLastProcessedRange(LastProcessedRange storage.BlobRange) {
	logFile.LastProcessedRange  = LastProcessedRange
}

func (logFile *AzureAppGwFirewallLogFile) SetLastProcessedTimeStamp(LastProcessedTimeStamp int64) {
	logFile.LastProcessedTimeStamp = LastProcessedTimeStamp
}

type AzureAppGwFirewallAccessLog struct {
	Records AzureAppGwFirewallEventRecords `json:"records"`
	AzureEventRecords []AzureEventRecord
}

func (log *AzureAppGwFirewallAccessLog) GetRecords() []AzureEventRecord {
	if log.AzureEventRecords == nil {
		log.AzureEventRecords = make([]AzureEventRecord, len(log.Records))
		for i, v := range log.Records {
			log.AzureEventRecords[i] = &v
		}
	}
	return log.AzureEventRecords
}

func NewAzureAppGwFirewallLogFile(blob storage.Blob) (AzureLogFile, error) {
	nsgLogFile := AzureAppGwFirewallLogFile{}
	nsgLogFile.Blob = blob
	nsgLogFile.Name = blob.Name
	nsgLogFile.Etag = blob.Properties.Etag
	nsgLogFile.LastModified = time.Time(blob.Properties.LastModified)

	logTime, err := getAppGwFirewallLogTimeFromName(blob.Name)
	nsgLogFile.LogTime = logTime

	nsgName, err := getLoggedResourceName(blob.Name)
	nsgLogFile.LoggedResourceName = nsgName

	return &nsgLogFile, err
}

func NewAzureAppGwFirewallLogFileFromEventLog(eventLog *AzureAppGwFirewallAccessLog) (AzureAppGwFirewallLogFile, error) {
	nsgLogFile := AzureAppGwFirewallLogFile{}
	nsgLogFile.AzureAppGwFirewallAccessLog = eventLog
	if len(eventLog.GetRecords()) == 0 {
		return AzureAppGwFirewallLogFile{}, nil
	}
	record := eventLog.GetRecords()[0]
	if !record.IsInitialized() {
		record.InitRecord()
	}

	nsgLogFile.Name = record.getSourceFileName()
	nsgLogFile.LastModified = time.Time(record.GetTime())

	nsgLogFile.Logger().Info("**********The name", nsgLogFile.Name)
	logTime, err := getAppGwFirewallLogTimeFromName(nsgLogFile.Name)

	nsgLogFile.LogTime = logTime

	nsgLogFile.LoggedResourceName = record.GetLogSourceName()

	return nsgLogFile, err
}

func (logFile *AzureAppGwFirewallLogFile) ShortName() string {
	logTime := logFile.LogTime.Format("2006-01-02-15")
	return fmt.Sprintf("%s-%s", logFile.LoggedResourceName, logTime)
}

func (logFile *AzureAppGwFirewallLogFile) GetName() string {
	return logFile.Name
}

func (logFile *AzureAppGwFirewallLogFile) GetNsgName() string {
	return logFile.LoggedResourceName
}

func (logFile *AzureAppGwFirewallLogFile) GetEtag() string {
	return logFile.Etag
}

func (logFile *AzureAppGwFirewallLogFile) GetLogTime() time.Time {
	return logFile.LogTime
}

func (logFile *AzureAppGwFirewallLogFile) GetAzureEventLog() AzureEventLog {
	return logFile.AzureAppGwFirewallAccessLog
}

func (logFile *AzureAppGwFirewallLogFile) GetLastProcessedRecord() time.Time {
	return logFile.LastProcessedRecord
}

func (logFile *AzureAppGwFirewallLogFile) GetLastProcessedTimeStamp() int64 {
	return logFile.LastProcessedTimeStamp
}

func (logFile *AzureAppGwFirewallLogFile) GetLastRecordCount() int {
	return logFile.LastRecordCount
}

func (logFile *AzureAppGwFirewallLogFile) GetLastModified() time.Time {
	return logFile.LastModified
}

func (logFile *AzureAppGwFirewallLogFile) GetLastProcessedRange() storage.BlobRange {
	return logFile.LastProcessedRange
}

func (logFile *AzureAppGwFirewallLogFile) GetBlob() storage.Blob {
	return logFile.Blob
}

func (logFile *AzureAppGwFirewallLogFile) LoadBlob() error {
	blobRange := storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	return logFile.LoadBlobRange(blobRange)
}

// Primary function for loading the storage.Blob object into an NsgLog
// Range is a set of byte offsets for reading the contents.
func (logFile *AzureAppGwFirewallLogFile) LoadBlobRange(blobRange storage.BlobRange) error {
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
func (logFile *AzureAppGwFirewallLogFile) LoadAzureNsgEventRecords(payload []byte) error {
	err := json.Unmarshal(payload, &logFile.AzureAppGwFirewallAccessLog)
	return err
}

// Provides a github.com/sirupsen/logrus template .
func (logFile *AzureAppGwFirewallLogFile) Logger() *log.Entry {
	return log.WithFields(log.Fields{
		"ShortName":           logFile.ShortName(),
		"LastProcessedRecord": logFile.LastProcessedRecord,
		"LastModified":        logFile.LastModified,
		"Nsg":                 logFile.LoggedResourceName,
	})
}

func getAppGwFirewallLogTimeFromName(name string) (time.Time, error) {
	nameTokens := AppGwFirewallFileRegExp.FindStringSubmatch(name)

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


func (logFile *AzureAppGwFirewallLogFile) getUnprocessedBlobRange() storage.BlobRange {
	var blobRange storage.BlobRange
	if logFile.LastProcessedRange.End != 0 {
		blobRange = storage.BlobRange{Start: logFile.LastProcessedRange.End, End: uint64(logFile.Blob.Properties.ContentLength)}
	} else {
		blobRange = storage.BlobRange{Start: 0, End: uint64(logFile.Blob.Properties.ContentLength)}
	}
	return blobRange
}
