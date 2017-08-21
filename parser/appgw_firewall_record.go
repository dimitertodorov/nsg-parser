package parser

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"regexp"
	"time"
	"encoding/json"
)

var (
	//1 = Subscription ID, 2 = Resource Group, 3 = Application Gateway
	AppGwFirewallRecordRegExp = regexp.MustCompile(`.*SUBSCRIPTIONS\/(.*)\/RESOURCEGROUPS\/(.*)\/PROVIDERS\/.*APPLICATIONGATEWAYS\/(.*)[\/]?[.*]*`)
)

type AzureAppGwFirewallEventRecords []AzureAppGwFirewallEventRecord

type AzureAppGwFirewallEventRecord struct {
	ResourceID     string    `json:"resourceId"`
	OperationName  string    `json:"operationName"`
	Time           time.Time `json:"time"`
	Category       string    `json:"category"`
	subscriptionId string
	resourceGroup  string
	AppGwName        string
	initialized    bool
	Properties     map[string]interface{} `json:"properties"`
}

func (record *AzureAppGwFirewallEventRecord) GetLogSourceName() string {
	return record.AppGwName
}

func (record *AzureAppGwFirewallEventRecord) GetTime() time.Time {
	return record.Time
}

func (record *AzureAppGwFirewallEventRecord) IsInitialized() bool {
	return record.initialized
}


func (record *AzureAppGwFirewallEventRecord) InitRecord() {
	nameTokens := AppGwFirewallRecordRegExp.FindStringSubmatch(record.ResourceID)
	if len(nameTokens) != 4 {
		log.Error(errResourceIdName)
		record.initialized = true
		return
	}
	record.subscriptionId = nameTokens[1]
	record.resourceGroup = nameTokens[2]
	record.AppGwName = nameTokens[3]
}

// Create a CEF Event Skeleton
func (record *AzureAppGwFirewallEventRecord) NewCEFEvent() CEFEvent {
	event := NewAzureCEFEventForProduct("Azure Application Gateway")
	event.Name = record.AppGwName
	event.DeviceEventClassId = record.Category
	event.Extension["cs2"] = record.AppGwName
	event.Extension["cs2label"] = "Azure Application Gateway"
	event.Extension["cs3"] = record.subscriptionId
	event.Extension["cs3label"] = "Subscription ID"
	event.Extension["cs4"] = record.resourceGroup
	event.Extension["cs4label"] = "Resource Group"
	return event
}

func (record *AzureAppGwFirewallEventRecord) GetCEFList(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	if !record.initialized {
		record.InitRecord()
	}
	switch record.OperationName {
	case "ApplicationGatewayFirewall":
		return record.convertAppGatewayFirewallEventsToCEF(options)
	default:
		return []*CEFEvent{}, []error{}
	}
}

func (record *AzureAppGwFirewallEventRecord) convertAppGatewayFirewallEventsToCEF(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	var events []*CEFEvent
	var errors []error
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered in convertAppGatewayFirewallEventsToCEF", r)
		}
	}()
	if record.Time.After(options.StartTime) {
		event := record.NewCEFEvent()

		event.Time = record.Time

		event.Extension["src"] = record.Properties["clientIp"].(string)
		cp := record.Properties["clientPort"]
		cpstr := fmt.Sprintf("%v", cp)
		event.Extension["spt"] = cpstr
		requestURI := record.Properties["requestUri"]
		requestURiStr := requestURI.(string)
		event.Extension["cs1"] = requestURiStr
		event.Extension["cs1label"] = "requestUri"
		event.Extension["act"] = record.Properties["action"].(string)
		// the issue detected
		event.Extension["cs5"] = record.Properties["message"].(string)
		event.Extension["cs5label"] = "message"
		event.Extension["cs6"] = record.Properties["instanceId"].(string)
		event.Extension["cs6label"] = "instanceId"
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

func (slice AzureAppGwFirewallEventRecords) Len() int {
	return len(slice)
}

func (slice AzureAppGwFirewallEventRecords) Less(i, j int) bool {
	return slice[i].Time.Before(slice[j].Time)
}

func (slice AzureAppGwFirewallEventRecords) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (slice AzureAppGwFirewallEventRecords) After(afterTime time.Time) AzureAppGwFirewallEventRecords {
	var returnRecords AzureAppGwFirewallEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

func (slice AzureAppGwFirewallEventRecords) Before(afterTime time.Time) AzureAppGwFirewallEventRecords {
	var returnRecords AzureAppGwFirewallEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

func (record *AzureAppGwFirewallEventRecord) getSourceFileName() string {
	fileTime := record.Time.Format("y=2006/m=01/d=02/h=15")
	return fmt.Sprintf("resourceId=%s/%s/m=00/PT1H.json", record.ResourceID, fileTime)
}

func (record *AzureAppGwFirewallEventRecord) getSourceContainerName() string {
	switch record.OperationName {
	case "ApplicationGatewayFirewall":
		return "insights-logs-applicationgatewayfirewalllog"
	default:
		return ""
	}
}
