package parser

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"time"
)

type FileClient struct {
	DataPath string
}

func (client FileClient) ProcessNsgLogFile(logFile AzureLogFile, resultsChan chan AzureLogFile) error {
	var fileName string
	blobRange := logFile.getUnprocessedBlobRange()
	err := logFile.LoadBlobRange(blobRange)
	if err != nil {
		log.Error(err)
		return err
	}

	events := []*CEFEvent{}
	for _, record := range logFile.GetAzureEventLog().GetRecords() {
		cefEvents, _ := record.GetCEFList(GetCEFEventListOptions{StartTime: logFile.GetLastProcessedRecord()})
		events = append(events, cefEvents...)
	}

	logCount := len(events)
	if logCount == 0 {
		logFile.Logger().Info("0 CEF Events extracted.")
		return nil
	}
	startTimeStamp := events[0].Time.Unix()
	endTimeStamp := events[logCount-1].Time.Unix()
	bm := NsgFileRegExp.FindStringSubmatch(logFile.GetBlob().Name)
	if len(bm) == 7 {
		fileName = fmt.Sprintf("nsgLog-%s-%s%s%s%s%s", bm[1], bm[2], bm[3], bm[4], bm[5], bm[6])
	} else {
		return fmt.Errorf("error in Blob.Name, expected 7 tokens. Got %d. Name: %s", len(bm), logFile.GetBlob().Name)
	}
	fileName = fmt.Sprintf("%s-%d-%d.json", fileName, startTimeStamp, endTimeStamp)
	outJson, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("error marshalling to json %s", err)
	}
	path := filepath.Join(client.DataPath, fileName)
	err = ioutil.WriteFile(path, outJson, 0666)

	logFile.SetLastProcessed(time.Now())
	logFile.SetLastRecordCount(len(logFile.GetAzureEventLog().GetRecords()))
	logFile.SetLastProcessedRecord(logFile.GetAzureEventLog().GetRecords()[logFile.GetLastRecordCount()-1].GetTime())
	logFile.SetLastProcessedRange(blobRange)

	resultsChan <- logFile
	return nil
}
