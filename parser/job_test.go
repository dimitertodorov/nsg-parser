package parser

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"fmt"
)

type MockClient struct{}

func (client MockClient) ProcessNsgLogFile(logFile *AzureNsgLogFile, resultsChan chan AzureNsgLogFile) error {
	events := []*CEFEvent{}
	for _, record := range logFile.AzureNsgEventLog.Records {
		cefEvents, _ := record.GetCEFList(GetCEFEventListOptions{})
		events = append(events, cefEvents...)
	}
	recordCount := len(logFile.AzureNsgEventLog.Records)
	logFile.LastProcessed = logFile.AzureNsgEventLog.Records[recordCount-1].Time
	logFile.LastRecordCount = recordCount
	logFile.LastProcessedRecord = logFile.AzureNsgEventLog.Records[logFile.LastRecordCount-1].Time
	resultsChan <- *logFile
	return nil
}

func TestNewJob(t *testing.T) {
	_, err := NewJob(&JobOptions{}, ProcessStatus{}, &AzureClient{}, MockClient{})
	if err != nil {
		t.Fatalf("got error creating job %s", err)
	}
}

func TestJobRun(t *testing.T) {
	for testKey, tt := range fileTests {
		t.Run(fmt.Sprintf("%s", testKey), func(t *testing.T) {
			client := MockClient{}
			logFile := loadTestLogFile(tt.testFile, t)
			fileName := logFile.AzureNsgEventLog.Records[0].getSourceFileName()
			processStatus := ProcessStatus{fileName: &logFile}
			job, err := NewJob(&JobOptions{}, processStatus, &AzureClient{}, client)
			if err != nil {
				t.Fatalf("got error creating job %s", err)
			}
			job.sideLoadLogFiles()
			job.LoadTasks()
			job.Run()
			assert.Equal(t, tt.expectedCount, job.ProcessStatus[fileName].LastRecordCount, "filename did not match")
		})
	}
}

func (job *Job) sideLoadLogFiles() {
	for _, logFile := range job.ProcessStatus {
		job.LogFiles = append(job.LogFiles, logFile)
	}
}
