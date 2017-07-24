package parser

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"regexp"
	"time"
	"strconv"
	"encoding/json"
)

var (
	//1 = Subscription ID, 2 = Resource Group, 3 = Application Gateway
	AppGwRecordRegExp = regexp.MustCompile(`.*SUBSCRIPTIONS\/(.*)\/RESOURCEGROUPS\/(.*)\/PROVIDERS\/.*APPLICATIONGATEWAYS\/(.*)[\/]?[.*]*`)
)

type AzureAppGwEventRecords []AzureAppGwEventRecord

type AzureAppGwEventRecord struct {
	ResourceID     string    `json:"resourceId"`
	OperationName  string    `json:"operationName"`
	Time           time.Time `json:"time"`
	Category       string    `json:"category"`
	subscriptionId string
	resourceGroup  string
	appGwName        string
	initialized    bool
	Properties     map[string]interface{} `json:"properties"`
}

func (record *AzureAppGwEventRecord) GetLogSourceName() string {
	return record.appGwName
}

func (record *AzureAppGwEventRecord) GetTime() time.Time {
	return record.Time
}

func (record *AzureAppGwEventRecord) IsInitialized() bool {
	return record.initialized
}


func (record *AzureAppGwEventRecord) InitRecord() {
	nameTokens := AppGwRecordRegExp.FindStringSubmatch(record.ResourceID)
	if len(nameTokens) != 4 {
		log.Error(errResourceIdName)
		record.initialized = true
		return
	}
	record.subscriptionId = nameTokens[1]
	record.resourceGroup = nameTokens[2]
	record.appGwName = nameTokens[3]
}

// Create a CEF Event Skeleton
func (record *AzureAppGwEventRecord) NewCEFEvent() CEFEvent {
	event := NewAzureCEFEventForProduct("Azure Application Gateway")
	event.Name = record.appGwName
	event.DeviceEventClassId = record.Category
	event.Extension["cs2"] = record.appGwName
	event.Extension["cs2label"] = "Azure Application Gateway"
	event.Extension["cs3"] = record.subscriptionId
	event.Extension["cs3label"] = "Subscription ID"
	event.Extension["cs4"] = record.resourceGroup
	event.Extension["cs4label"] = "Resource Group"
	return event
}

func (record *AzureAppGwEventRecord) GetCEFList(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	if !record.initialized {
		record.InitRecord()
	}
	switch record.OperationName {
	case "ApplicationGatewayAccess":
		return record.convertApplicationGatewayEventsToCEF(options)
	default:
		return []*CEFEvent{}, []error{}
	}
}

func (record *AzureAppGwEventRecord) convertApplicationGatewayEventsToCEF(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	var events []*CEFEvent
	var errors []error
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered in convertApplicationGatewayEventsToCEF", r)
		}
	}()
	if record.Time.After(options.StartTime) {
		event := record.NewCEFEvent()

		event.Time = record.Time

		event.Extension["src"] = record.Properties["clientIP"].(string)
		event.Extension["spt"] = strconv.Itoa(int(record.Properties["clientPort"].(float64)))
		event.Extension["request"] = record.Properties["requestUri"].(string)
		event.Extension["requestMethod"] = record.Properties["httpMethod"].(string)
		event.Extension["act"] = record.Properties["httpMethod"].(string)
		// include all properties into CEF message
		jsonBytes, marshal_error := json.Marshal(record.Properties)
		if marshal_error != nil {
			log.Errorf("marshal record failed: %v", marshal_error)
		}
		event.Extension["msg"] = string(jsonBytes)

		events = append(events, &event)
	}
	return events, errors
}

func (slice AzureAppGwEventRecords) Len() int {
	return len(slice)
}

func (slice AzureAppGwEventRecords) Less(i, j int) bool {
	return slice[i].Time.Before(slice[j].Time)
}

func (slice AzureAppGwEventRecords) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (slice AzureAppGwEventRecords) After(afterTime time.Time) AzureAppGwEventRecords {
	var returnRecords AzureAppGwEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

func (slice AzureAppGwEventRecords) Before(afterTime time.Time) AzureAppGwEventRecords {
	var returnRecords AzureAppGwEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

func (record *AzureAppGwEventRecord) getSourceFileName() string {
	fileTime := record.Time.Format("y=2006/m=01/d=02/h=15")
	return fmt.Sprintf("resourceId=%s/%s/m=00/PT1H.json", record.ResourceID, fileTime)
}

func (record *AzureAppGwEventRecord) getSourceContainerName() string {
	switch record.OperationName {
	case "ApplicationGatewayAccess":
		return "insights-logs-applicationgatewayaccesslog"
	default:
		return ""
	}
}
