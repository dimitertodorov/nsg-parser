package parser

import (
	"encoding/json"
	"fmt"
	syslog "github.com/RackSec/srslog"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var testEvents = []struct {
	event          CefEvent
	expectedFormat string
}{
	{
		event: createTestEvent(map[string]string{
			"cs1":             "UserRule_HTTP",
			"deviceDirection": "0",
			"dmac":            "00:0D:3A:F3:38:54",
			"dpt":             "80",
			"dst":             "10.193.160.4",
			"outcome":         "Allow",
			"proto":           "TCP",
			"spt":             "15425",
			"src":             "10.199.1.8",
		}),
		expectedFormat: "%s|CEF:0|Microsoft|Azure NSG|1|nsg-flow|nsg-flow|0|cs1=UserRule_HTTP deviceDirection=0 dmac=00:0D:3A:F3:38:54 dpt=80 dst=10.193.160.4 outcome=Allow proto=TCP spt=15425 src=10.199.1.8",
	},
	{
		event: createTestEvent(map[string]string{
			"cs1": "UserRule_HTTP",
			"act": `check backslash \`,
		}),
		expectedFormat: "%s|CEF:0|Microsoft|Azure NSG|1|nsg-flow|nsg-flow|0|act=check backslash \\ cs1=UserRule_HTTP",
	},
	{
		event: createTestEvent(map[string]string{
			"cs1": "UserRule_HTTP",
			"act": `check equals =`,
		}),
		expectedFormat: "%s|CEF:0|Microsoft|Azure NSG|1|nsg-flow|nsg-flow|0|act=check equals \\= cs1=UserRule_HTTP",
	},
	{
		event: createTestEvent(map[string]string{
			"cs1": "UserRule_HTTP",
			"act": "check multiline \n sadasd",
		}),
		expectedFormat: "%s|CEF:0|Microsoft|Azure NSG|1|nsg-flow|nsg-flow|0|act=check multiline \n sadasd cs1=UserRule_HTTP",
	},
}

func createTestEvent(extensions map[string]string) CefEvent {
	event := NewNsgCefEvent()
	event.Time = time.Now()
	event.Name = EventClassIdFlow
	event.DeviceEventClassId = EventClassIdFlow
	event.Severity = 0
	event.Extension = extensions
	return event
}

func TestCEFSyslogFormatter(t *testing.T) {
	out := CEFSyslogFormatter(syslog.LOG_ERR, "hostname", "tag", "CEF")
	expected := fmt.Sprintf("%s %s %s",
		time.Now().Format(CefTimeFormat), "hostname", "CEF")
	assert.Equal(t, expected, out, "Base CEF Message should get formatted properly")
}

func TestCEFSyslogFormatterWithTime(t *testing.T) {
	out := CEFSyslogFormatter(syslog.LOG_ERR, "hostname", "tag", "Jan 02 15:04:05|CEF")
	expected := fmt.Sprintf("%s %s %s",
		"Jan 02 15:04:05", "hostname", "CEF")
	assert.Equal(t, expected, out, "CEF Formatter should use specified timestamp when provided.")
}

func TestConvertNsgRecordToCEFEvents(t *testing.T) {
	testTime, _ := time.Parse(timeLayout, "06/09 20:33:00 GMT 2017")
	options := GetCefEventListOptions{
		StartTime: testTime,
	}
	eventList, _ := GetCefEventListFromNsg(&sampleNsgLog, options)
	assert.Equal(t, 1513, len(eventList.Events), "should filter out older records")
}

func TestToSyslogEvent(t *testing.T) {
	for _, tt := range testEvents {
		slText, err := tt.event.SyslogText()
		assert.Nil(t, err, "got error converting to syslog")
		expectedCef := fmt.Sprintf(tt.expectedFormat, tt.event.Time.Format(CefTimeFormat))
		assert.Equal(t, expectedCef, slText, "should convert to CEF Properly")
	}
}

func TestMarshal(t *testing.T) {
	for _, tt := range testEvents {
		_, err := json.MarshalIndent(tt.event, "", "    ")
		assert.Nil(t, err, "no error")
	}
}
