package parser

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	//1 = Subscription ID, 2 = Resource Group, 3 = NSG
	RecordRegExp = regexp.MustCompile(`.*SUBSCRIPTIONS\/(.*)\/RESOURCEGROUPS\/(.*)\/PROVIDERS\/.*NETWORKSECURITYGROUPS\/(.*)[\/]?[.*]*`)
)

type AzureLogQueryOptions struct {
	BeginTime time.Time
	EndTime   time.Time
}

type AzureNsgEventRecords []AzureNsgEventRecord

type AzureNsgEventRecord struct {
	Time           time.Time `json:"time"`
	SystemID       string    `json:"systemId"`
	Category       string    `json:"category"`
	ResourceID     string    `json:"resourceId"`
	OperationName  string    `json:"operationName"`
	subscriptionId string
	resourceGroup  string
	nsgName        string
	initialized    bool
	Properties     map[string]interface{} `json:"properties"`
}

func (record *AzureNsgEventRecord) InitRecord() {
	nameTokens := RecordRegExp.FindStringSubmatch(record.ResourceID)
	if len(nameTokens) != 4 {
		log.Error(errResourceIdName)
		record.initialized = true
		return
	}
	record.subscriptionId = nameTokens[1]
	record.resourceGroup = nameTokens[2]
	record.nsgName = nameTokens[3]
}

// Create a CEF Event Skeleton
func (record *AzureNsgEventRecord) NewCEFEvent() CEFEvent {
	event := NewNsgCEFEvent()
	event.Name = record.Category
	event.DeviceEventClassId = record.OperationName
	event.Extension["deviceExternalId"] = record.SystemID
	event.Extension["cs2"] = record.nsgName
	event.Extension["cs2label"] = "Azure NSG"
	event.Extension["cs3"] = record.subscriptionId
	event.Extension["cs3label"] = "Subscription ID"
	event.Extension["cs4"] = record.resourceGroup
	event.Extension["cs4label"] = "Resource Group"
	return event
}

func (record *AzureNsgEventRecord) GetCEFList(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	if !record.initialized {
		record.InitRecord()
	}
	switch record.OperationName {
	case "NetworkSecurityGroupFlowEvents":
		return record.convertNetworkSecurityGroupFlowEventsToCEF(options)
	case "NetworkSecurityGroupEvents":
		return record.convertNetworkSecurityGroupEventsToCEF(options)
	default:
		return []*CEFEvent{}, []error{}
	}
}

func (record *AzureNsgEventRecord) convertNetworkSecurityGroupFlowEventsToCEF(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	var events []*CEFEvent
	var errors []error
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered in convertNetworkSecurityGroupFlowEventsToCEF", r)
		}
	}()
	if record.Time.After(options.StartTime) {
		flows := record.Properties["flows"].([]interface{})
		for _, f := range flows {
			flow := f.(map[string]interface{})
			flowRule := flow["rule"].(string)
			for _, subFlow := range flow["flows"].([]interface{}) {
				thisSubFlow := subFlow.(map[string]interface{})
				flowTuples := thisSubFlow["flowTuples"].([]interface{})
				for _, flowTuple := range flowTuples {
					tuple := flowTuple.(string)
					event := record.NewCEFEvent()

					event.Extension["cs1"] = flowRule
					event.Extension["cs1label"] = "Rule Name"

					//Tuple-Specific properties below here.
					tuples := strings.Split(tuple, ",")
					if len(tuples) != 8 {
						errors = append(errors, fmt.Errorf("unexpected # tokens in tuple %s. expected 8", flowTuple))
						continue
					}

					epochTime, err := strconv.ParseInt(tuples[0], 10, 64)
					if err != nil {
						errors = append(errors, err)
					}
					event.Time = time.Unix(epochTime, 0)

					event.Extension["start"] = fmt.Sprintf("%d", 1000*epochTime)
					event.Extension["src"] = tuples[1]
					event.Extension["dst"] = tuples[2]
					event.Extension["spt"] = tuples[3]
					event.Extension["dpt"] = tuples[4]

					event.Extension["proto"] = protocolMap[tuples[5]]

					// The MAC address captured is that of the VM in Azure.
					// Set destination or source MAC depending on direction of detected flow.
					flowDirection := cefDirectionMap[tuples[6]]
					event.Extension["deviceDirection"] = fmt.Sprintf("%d", flowDirection)
					switch flowDirection {
					case 0:
						event.Extension["dmac"] = formatMac(thisSubFlow["mac"].(string))
					case 1:
						event.Extension["smac"] = formatMac(thisSubFlow["mac"].(string))
					}

					flowOutcome := cefOutcomeMap[tuples[7]]
					event.Extension["categoryOutcome"] = flowOutcome
					switch flowOutcome {
					case "Allow":
						event.Severity = 0
					case "Deny":
						event.Severity = 6
					default:
						event.Severity = 4
						event.Extension["categoryOutcome"] = "Unknown"
					}

					events = append(events, &event)
				}
			}
		}
	}
	return events, errors
}

func (slice AzureNsgEventRecords) Len() int {
	return len(slice)
}

func (slice AzureNsgEventRecords) Less(i, j int) bool {
	return slice[i].Time.Before(slice[j].Time)
}

func (slice AzureNsgEventRecords) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (slice AzureNsgEventRecords) After(afterTime time.Time) AzureNsgEventRecords {
	var returnRecords AzureNsgEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

func (slice AzureNsgEventRecords) Before(afterTime time.Time) AzureNsgEventRecords {
	var returnRecords AzureNsgEventRecords
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

// TODO: Map NetworkSecurityGroupEvent to CEF
func (record *AzureNsgEventRecord) convertNetworkSecurityGroupEventsToCEF(options GetCEFEventListOptions) ([]*CEFEvent, []error) {
	return []*CEFEvent{}, []error{}
}

func (record *AzureNsgEventRecord) getSourceFileName() string {
	fileTime := record.Time.Format("y=2006/m=01/d=02/h=15")
	return fmt.Sprintf("resourceId=%s/%s/m=00/PT1H.json", record.ResourceID, fileTime)
}

func (record *AzureNsgEventRecord) getSourceContainerName() string {
	switch record.OperationName {
	case "NetworkSecurityGroupFlowEvents":
		return "insights-logs-networksecuritygroupflowevent"
	case "NetworkSecurityGroupEvents":
		return "insights-logs-networksecuritygroupevent"
	default:
		return ""
	}
}
