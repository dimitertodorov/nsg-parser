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

/*var recordErrorTests = map[string][]struct {
	record            []byte
	errorCount        int
	firstErrorMessage string
}{
	"NetworkSecurityGroupFlowEvents": {
		{
			record: []byte(`{
  "time": "2017-06-09T20:07:33.837Z",
  "systemId": "fe485b0f-4e32-4dc2-ad20-ba20243985d3",
  "category": "NetworkSecurityGroupFlowEvent",
  "resourceId": "/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME-NSG",
  "operationName": "NetworkSecurityGroupFlowEvents",
  "properties": {
    "Version": 1,
    "flows": [
      {
        "rule": "DefaultRule_AllowVnetOutBound",
        "flows": [
          {
            "mac": "000D3AF33854",
            "flowTuples": [
              "1497038813,10.193.160.4,40.85.232.72,46010",
              "1497038814,10.193.160.4,40.85.232.72,46010,443,T,O,A"
            ]
          }
        ]
      }
    ]
  }
}`),
			errorCount:        1,
			firstErrorMessage: "unexpected # tokens in tuple 1497038813,10.193.160.4,40.85.232.72,46010. expected 8",
		},
		{
			record: []byte(`{
  "time": "2017-06-09T20:07:33.837Z",
  "systemId": "fe485b0f-4e32-4dc2-ad20-ba20243985d3",
  "category": "NetworkSecurityGroupFlowEvent",
  "resourceId": "/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME-NSG",
  "operationName": "NetworkSecurityGroupFlowEvents",
  "properties": {
    "Version": 1,
    "flows": [
      {
        "rule": "DefaultRule_AllowVnetOutBound",
        "flows": [
          {
            "mac": "000D3AF33854",
            "flowTuples": [
              "abcde,10.193.160.4,40.85.232.72,46010,443,T,O,A",
              "1497038814,10.193.160.4,40.85.232.72,46010,443,T,O,A"
            ]
          }
        ]
      }
    ]
  }
}`),
			errorCount:        1,
			firstErrorMessage: `strconv.ParseInt: parsing "abcde": invalid syntax`,
		},
	},
}

var miscRecordTests = map[string][]struct {
	record              []byte
	sourceFileName      string
	sourceContainerName string
}{
	"NetworkSecurityGroupFlowEvents": {
		{
			record: []byte(`{
  "time": "2017-06-09T20:07:33.837Z",
  "systemId": "fe485b0f-4e32-4dc2-ad20-ba20243985d3",
  "category": "NetworkSecurityGroupFlowEvent",
  "resourceId": "/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME-NSG",
  "operationName": "NetworkSecurityGroupFlowEvents",
  "properties": {
    "Version": 1,
    "flows": [
      {
        "rule": "DefaultRule_AllowVnetOutBound",
        "flows": [
          {
            "mac": "000D3AF33854",
            "flowTuples": [
              "1497038813,10.193.160.4,40.85.232.72,46010,443,T,O,A",
              "1497038814,10.193.160.4,40.85.232.72,46010,443,T,O,A"
            ]
          }
        ]
      }
    ]
  }
}`),
			sourceFileName:      "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME-NSG/y=2017/m=06/d=09/h=20/m=00/PT1H.json",
			sourceContainerName: "insights-logs-networksecuritygroupflowevent",
		},
	},
}
*/
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
