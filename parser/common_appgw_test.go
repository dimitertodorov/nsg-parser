package parser

import (
	"testing"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
)

var (
	sampleaAppGwName = "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/APPGWNAME/y=2017/m=07/d=21/h=15/m=00/PT1H.json"

	sampleAppGwProcessStatusFile = "../testdata/process_status_sample.json"

//	timeAppGwLayout   = "01/02 15:04:05 GMT 2006"
)

var fileAppGwTests = map[string]struct {
	testFile              string
	expectedOperation     string
	expectedCount         int
	expectedCEFEventCount int
	afterTime             string
	afterCount            int
	sourceFileName        string
	sourceContainerName   string
}{
	"ApplicationGatewayEvents": {
		testFile:              "app_gateway_access.json",
		expectedOperation:     "ApplicationGatewayAccess",
		expectedCount:         89,
		expectedCEFEventCount: 89,
		afterTime:             "06/22 00:40:00 GMT 2017",
		afterCount:            52,
		sourceFileName:        "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/APPGWNAME/y=2017/m=07/d=21/h=15/m=00/PT1H.json",
		sourceContainerName:   "potato",
	},
}


func loadAppGwTestFile(name string, t *testing.T) AzureAppGwAccessLog {
	logs := AzureAppGwAccessLog{}
	file, err := ioutil.ReadFile(filepath.Join(testDataPath, name))
	if err != nil {
		t.Fatalf("got error loading testfile %s %s", name, err)
		return logs
	}
	err = json.Unmarshal(file, &logs)
	if err != nil {
		t.Fatalf("got error unmarshalling testfile %s %s", name, err)
		return logs
	}
	return logs
}


func loadTestAppGwLogFile(name string, t *testing.T) AzureLogFile {
	eventLog := loadAppGwTestFile(name, t)
	logFile, err := NewAzureAppGwLogFileFromEventLog(&eventLog)
	if err != nil {
		t.Fatalf("got error loading testfile into AzureNsgLogFile %s %s", name, err)
	}
	return &logFile
}
