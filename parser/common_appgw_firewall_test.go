package parser

import (
	"testing"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
)

var (
	sampleaAppGwFirewallName = "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/APPGWNAME/y=2017/m=07/d=21/h=15/m=00/PT1H.json"

	sampleAppGwFirewallProcessStatusFile = "../testdata/process_status_sample.json"

//	timeAppGwLayout   = "01/02 15:04:05 GMT 2006"
)

var fileAppGwFirewallTests = map[string]struct {
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
		testFile:              "app_gateway_firewall_log.json",
		expectedOperation:     "ApplicationGatewayFirewall",
		expectedCount:         327,
		expectedCEFEventCount: 327,
		afterTime:             "06/22 00:40:00 GMT 2017",
		afterCount:            52,
		sourceFileName:        "resourceId=/SUBSCRIPTIONS/C10F1FB1-D7ED-40CD-91A8-0D3A82A55F8D/RESOURCEGROUPS/SDCCDEV01RGP03/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/SDCTS1WAF001/y=2017/m=07/d=21/h=17/m=00/PT1H.json",
		sourceContainerName:   "potato",
	},
}

var miscAppGwFirewallRecordTests = map[string][]struct {
	record              []byte
	sourceFileName      string
	sourceContainerName string
}{
	"NetworkSecurityGroupFlowEvents": {
		{
			record: []byte(`{
			 "resourceId": "/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/APPGWNAME",
			 "operationName": "ApplicationGatewayFirewall",
			 "time": "2017-07-21T17:33:20Z",
			 "category": "ApplicationGatewayFirewallLog",
			 "properties": {
  "instanceId": "ApplicationGatewayRole_IN_0",
  "clientIp": "52.237.25.113",
  "clientPort": "0",
  "requestUri": "/health.htm",
  "ruleSetType": "OWASP",
  "ruleSetVersion": "2.2.9",
  "ruleId": "960015",
  "message": "Request Missing an Accept Header",
  "action": "Detected",
  "site": "Global",
  "details": {
    "message": "Warning. Operator EQ matched 0 at REQUEST_HEADERS.",
    "data": "",
    "file": "base_rules/modsecurity_crs_21_protocol_anomalies.conf",
    "line": "47"
  }
}
		}`),
			sourceFileName:      "resourceId=/SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/APPLICATIONGATEWAYS/APPGWNAME/y=2017/m=07/d=21/h=17/m=00/PT1H.json",
			sourceContainerName: "insights-logs-applicationgatewayfirewalllog",
		},
	},
}


func loadAppGwFirewallTestFile(name string, t *testing.T) AzureAppGwFirewallAccessLog {
	logs := AzureAppGwFirewallAccessLog{}
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


func loadTestAppGwFirewallLogFile(name string, t *testing.T) AzureLogFile {
	eventLog := loadAppGwFirewallTestFile(name, t)
	logFile, err := NewAzureAppGwFirewallLogFileFromEventLog(&eventLog)
	if err != nil {
		t.Fatalf("got error loading testfile into AzureNsgLogFile %s %s", name, err)
	}
	return &logFile
}
