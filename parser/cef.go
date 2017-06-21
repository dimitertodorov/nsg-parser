package parser

import (
	"bytes"
	"fmt"
	syslog "github.com/RackSec/srslog"
	log "github.com/sirupsen/logrus"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Severity|[Extension]

const (
	EventClassIdFlow          = "nsg-flow"
	EventClassIdFlowAggregate = "nsg-flow-aggregate"
	CefTimeFormat             = "Jan 02 15:04:05"
)

var (
	CefVersion       = 0
	NsgDeviceVendor  = "Microsoft"
	NsgDeviceProduct = "Azure NSG"
	NsgDeviceVersion = "1"
)

type CefEvent struct {
	CefVersion         *int
	DeviceVendor       *string
	DeviceProduct      *string
	DeviceVersion      *string
	DeviceEventClassId string
	Time               time.Time
	Name               string
	Severity           int
	Extension          map[string]string
}

type CefEventList struct {
	Events []*CefEvent
}

type GetCefEventListOptions struct {
	StartTime time.Time
}

type CefSyslogClient struct {
	writer      *syslog.Writer
	template    template.Template
	initialized bool
}

var (
	cefTemplateText = `CEF:{{.CefVersion}}|{{.DeviceVendor}}|{{.DeviceProduct}}|{{.DeviceVersion}}|{{.DeviceEventClassId}}|{{.Name}}|{{.Severity}}{{.ExtensionText}}`
	eventWithTime   = regexp.MustCompile(`^(.*)\|(.*)$`)
	cefTemplate     template.Template
)

var protocolMap = map[string]string{
	"T": "TCP",
	"U": "UDP",
}

var directionMap = map[string]int{
	"I": 0,
	"O": 1,
}

var outcomeMap = map[string]string{
	"A": "Allow",
	"D": "Deny",
}

func init() {
	tpl, err := template.New("cefEventTemplate").Parse(cefTemplateText)
	if err != nil {
		log.Fatalf("error loading cef template: %s", err)
	}
	cefTemplate = *tpl
}

func NewNsgCefEvent() CefEvent {
	return CefEvent{
		CefVersion:    &CefVersion,
		DeviceVendor:  &NsgDeviceVendor,
		DeviceProduct: &NsgDeviceProduct,
		DeviceVersion: &NsgDeviceVersion,
		Extension:     make(map[string]string),
	}
}

func (event *CefEvent) SyslogText() (string, error) {
	var templateText bytes.Buffer
	err := cefTemplate.Execute(&templateText, event)
	if err != nil {
		return "", err
	}

	if event.Time != (time.Time{}) {
		return fmt.Sprintf("%s|%s", event.Time.Format(CefTimeFormat), templateText.String()), nil
	} else {
		return templateText.String(), nil
	}

	return "", nil
}

func (event *CefEvent) ExtensionText() (string, error) {
	var extensionText []byte

	keyCount := 0
	extensionKeys := make([]string, len(event.Extension))
	for k := range event.Extension {
		extensionKeys[keyCount] = k
		keyCount++
	}
	sort.Strings(extensionKeys)

	for _, key := range extensionKeys {
		value := event.Extension[key]
		if value != "" {
			encodedPair := fmt.Sprintf("%s=%s ", key, formatValue(value))
			extensionText = append(extensionText, []byte(encodedPair)...)
		}
	}

	if len(extensionText) != 0 {
		return fmt.Sprintf("|%s", strings.TrimSpace(string(extensionText[:]))), nil
	} else {
		return "", nil
	}
}

func formatValue(value string) string {
	value = strings.Replace(value, `=`, `\=`, -1)
	return value
}

// CEFSyslogFormatter provides a CEF Compliant message
// This implementation also extracts a timestamp if pre-pended to the message.
// If a timestamp is provided, the event time is set to that.
// Example: Sep 19 08:26:10 host CEF:0|Security|threatmanager|1.0|100|worm successfully stopped|10|src=10.0.0.1 dst=2.1.2.2 spt=1232
func CEFSyslogFormatter(_ syslog.Priority, hostname, _, content string) string {
	var msg string
	var timestamp string
	msgParts := eventWithTime.FindStringSubmatch(content)
	if len(msgParts) == 3 {
		timestamp = msgParts[1]
		content = msgParts[2]
	} else {
		timestamp = time.Now().Format(CefTimeFormat)
	}
	msg = fmt.Sprintf("%s %s %s",
		timestamp, hostname, content)
	return msg
}

func GetCefEventListFromNsg(nsgLog *NsgLog, options GetCefEventListOptions) (*CefEventList, error) {
	var eventList CefEventList
	var events []*CefEvent
	for _, record := range nsgLog.Records {
		if record.Time.After(options.StartTime) {
			for _, flow := range record.Properties.Flows {
				for _, subFlow := range flow.Flows {
					for _, flowTuple := range subFlow.FlowTuples {
						event := NewNsgCefEvent()
						tuples := strings.Split(flowTuple, ",")
						if len(tuples) != 8 {
							return &eventList, fmt.Errorf("unexpected tokens in tuple %s. expected 8", flowTuple)
						}
						epochTime, _ := strconv.ParseInt(tuples[0], 10, 64)
						event.Time = time.Unix(epochTime, 0)
						event.Name = EventClassIdFlow
						event.DeviceEventClassId = EventClassIdFlow
						event.Severity = 0

						event.Extension["start"] = fmt.Sprintf("%d", 1000*epochTime)

						event.Extension["cs1"] = flow.Rule

						event.Extension["src"] = tuples[1]
						event.Extension["dst"] = tuples[2]
						event.Extension["spt"] = tuples[3]
						event.Extension["dpt"] = tuples[4]

						event.Extension["proto"] = protocolMap[tuples[5]]
						flowDirection := directionMap[tuples[6]]
						switch flowDirection {
						case 0:
							event.Extension["dmac"] = formatMac(subFlow.Mac)
						case 1:
							event.Extension["smac"] = formatMac(subFlow.Mac)
						}
						event.Extension["deviceDirection"] = fmt.Sprintf("%d", flowDirection)
						event.Extension["outcome"] = outcomeMap[tuples[7]]
						events = append(events, &event)
					}
				}
			}
		}

	}
	eventList.Events = events
	return &eventList, nil
}

func (client *CefSyslogClient) Initialize(protocol, host, port string, azureClient *AzureClient) error {
	syslogWriter, err := syslog.Dial(protocol, fmt.Sprintf("%s:%s", host, port),
		syslog.LOG_ERR, "nsg-parser")
	if err != nil {
		log.Fatal(err)
		return err
	}

	syslogWriter.SetFormatter(CEFSyslogFormatter)

	client.template = cefTemplate
	client.writer = syslogWriter
	client.initialized = true

	azureClient.DestinationType = DestinationSyslog
	if err = azureClient.LoadProcessStatus(); err != nil {
		return err
	}
	return nil
}

func (client *CefSyslogClient) SendEvent(event CefEvent) error {
	if !client.initialized {
		return fmt.Errorf("uninitialized syslog client")
	}
	logText, err := event.SyslogText()
	if err != nil {
		return fmt.Errorf("event_format_error %s", err)
	}
	fmt.Fprintf(client.writer, "%s", logText)
	return nil
}

func (client CefSyslogClient) ProcessNsgLogFile(logFile *NsgLogFile, resultsChan chan NsgLogFile) error {
	blobRange := logFile.getUnprocessedBlobRange()
	err := logFile.LoadBlobRange(blobRange)
	if err != nil {
		log.Error(err)
		return err
	}

	cefEvents, err := GetCefEventListFromNsg(logFile.NsgLog, GetCefEventListOptions{StartTime: logFile.LastProcessedRecord})
	if err != nil {
		return err
	}

	logCount := len(cefEvents.Events)
	endTimeStamp := cefEvents.Events[logCount-1].Time.Unix()
	logFile.LastProcessedTimeStamp = endTimeStamp
	for _, nsgEvent := range cefEvents.Events {
		client.SendEvent(*nsgEvent)
	}

	logFile.LastProcessed = time.Now()
	logFile.LastRecordCount = len(logFile.NsgLog.Records)
	logFile.LastProcessedRecord = logFile.NsgLog.Records[logFile.LastRecordCount-1].Time
	logFile.LastProcessedRange = blobRange

	processedFlowCount.Inc(int64(logCount))

	resultsChan <- *logFile
	return nil
}
