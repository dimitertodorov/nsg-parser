package parser

import (
	"bytes"
	"fmt"
	syslog "github.com/RackSec/srslog"
	log "github.com/sirupsen/logrus"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

const (
	EventClassIdFlow = "nsg-flow"
	CEFTimeFormat    = "Jan 02 15:04:05"
)

var (
	CEFVersion       = 0
	NsgDeviceVendor  = "Microsoft"
	NsgDeviceProduct = "Azure NSG"
	AppGatewayDeviceProduct = "Azure Application Gateway"
	NsgDeviceVersion = "1"
)

type CEFEvent struct {
	CEFVersion         *int              `json:"cef_version"`
	DeviceVendor       *string           `json:"device_vendor"`
	DeviceProduct      *string           `json:"device_product"`
	DeviceVersion      *string           `json:"device_product"`
	DeviceEventClassId string            `json:"device_event_class_id"`
	Time               time.Time         `json:"time"`
	Name               string            `json:"name"`
	Severity           int               `json:"severity"`
	Extension          map[string]string `json:"extension"`
}

type CEFEventList struct {
	Events []*CEFEvent
}

type GetCEFEventListOptions struct {
	StartTime time.Time
}

type CEFSyslogClient struct {
	writer      *syslog.Writer
	template    template.Template
	initialized bool
}

var (
	cefTemplateText = `CEF:{{.CEFVersion}}|{{.DeviceVendor}}|{{.DeviceProduct}}|{{.DeviceVersion}}|{{.DeviceEventClassId}}|{{.Name}}|{{.Severity}}{{.ExtensionText}}`
	eventWithTime   = regexp.MustCompile(`^(.*)\|(CEF.*)`)
	cefTemplate     template.Template
)

var protocolMap = map[string]string{
	"T": "TCP",
	"U": "UDP",
}

var cefDirectionMap = map[string]int{
	"I": 0,
	"O": 1,
}

var cefOutcomeMap = map[string]string{
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

func NewAzureCEFEvent() CEFEvent {
	return CEFEvent{
		CEFVersion:    &CEFVersion,
		DeviceVendor:  &NsgDeviceVendor,
		DeviceProduct: &NsgDeviceProduct,
		DeviceVersion: &NsgDeviceVersion,
		Extension:     make(map[string]string),
	}
}

func NewAzureCEFEventForProduct(deviceProduct string) CEFEvent {
	return CEFEvent{
		CEFVersion:    &CEFVersion,
		DeviceVendor:  &NsgDeviceVendor,
		DeviceProduct: &deviceProduct,
		DeviceVersion: &NsgDeviceVersion,
		Extension:     make(map[string]string),
	}
}

func (event *CEFEvent) SyslogText() (string, error) {
	var templateText bytes.Buffer
	err := cefTemplate.Execute(&templateText, event)
	if err != nil {
		return "", err
	}

	if event.Time != (time.Time{}) {
		return fmt.Sprintf("%s|%s", event.Time.Format(CEFTimeFormat), templateText.String()), nil
	} else {
		return templateText.String(), nil
	}
}

func (event *CEFEvent) ExtensionText() (string, error) {
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
// This implementation also extracts a timestamp if pre-pended to the message
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
		timestamp = time.Now().Format(CEFTimeFormat)
	}
	msg = fmt.Sprintf("%s %s %s",
		timestamp, hostname, content)
	return msg
}

func (client *CEFSyslogClient) Initialize(protocol, host, port string) error {
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

	return nil
}

func (client *CEFSyslogClient) SendEvent(event CEFEvent) error {
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

func (client CEFSyslogClient) ProcessNsgLogFile(logFile AzureLogFile, resultsChan chan AzureLogFile) error {
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
	endTimeStamp := events[logCount-1].Time.Unix()
	logFile.SetLastProcessedTimeStamp(endTimeStamp)
	for _, nsgEvent := range events {
		client.SendEvent(*nsgEvent)
	}

	logFile.SetLastProcessed (time.Now())
	logFile.SetLastRecordCount (len(logFile.GetAzureEventLog().GetRecords()))
	logFile.SetLastProcessedRecord (logFile.GetAzureEventLog().GetRecords()[logFile.GetLastRecordCount()-1].GetTime())
	logFile.SetLastProcessedRange (blobRange)
	logFile.SetLastProcessedRange (blobRange)

	processedFlowCount.Inc(int64(logCount))

	resultsChan <- logFile
	return nil
}
