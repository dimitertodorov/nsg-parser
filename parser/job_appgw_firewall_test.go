package parser

import (
	"testing"
	"fmt"
	"github.com/stretchr/testify/assert"
)

type AppGwFirewallMockClient struct{}

func (client AppGwFirewallMockClient) ProcessAzureLogFile(logFile AzureLogFile, resultsChan chan AzureLogFile) error {
	events := []*CEFEvent{}
	for _, record := range logFile.GetAzureEventLog().GetRecords() {
		cefEvents, _ := record.GetCEFList(GetCEFEventListOptions{})
		events = append(events, cefEvents...)
	}
	recordCount := len(logFile.GetAzureEventLog().GetRecords())
	logFile.SetLastProcessed(logFile.GetAzureEventLog().GetRecords()[recordCount-1].GetTime())
	logFile.SetLastRecordCount (recordCount)
	logFile.SetLastProcessedRecord(logFile.GetAzureEventLog().GetRecords()[logFile.GetLastRecordCount()-1].GetTime())
	resultsChan <- logFile
	return nil
}

func AppGwFirewallTestNewJob(t *testing.T) {
	_, err := NewJob(&JobOptions{}, ProcessStatus{}, &AzureClient{}, AppGwFirewallMockClient{})
	if err != nil {
		t.Fatalf("got error creating job %s", err)
	}
}

func TestAppGwFirewallJobRun(t *testing.T) {
	for testKey, tt := range fileAppGwFirewallTests {
		t.Run(fmt.Sprintf("%s", testKey), func(t *testing.T) {
			client := AppGwFirewallMockClient{}
			logFile := loadTestAppGwFirewallLogFile(tt.testFile, t)
			fileName := logFile.GetAzureEventLog().GetRecords()[0].getSourceFileName()
			processStatus := ProcessStatus{fileName: createProcessStatusFromLogfile(logFile)}
			job, err := NewJob(&JobOptions{}, processStatus, &AzureClient{}, client)
			if err != nil {
				t.Fatalf("got error creating job %s", err)
			}
			job.LogFiles = append(job.LogFiles, logFile)
			//			job.sideLoadLogFiles()
			job.LoadTasks()
			job.Run()
			assert.Equal(t, tt.expectedCount, job.ProcessStatus[fileName].LastRecordCount, "filename did not match")
		})
	}
}

