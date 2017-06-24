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
	event          CEFEvent
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

func createTestEvent(extensions map[string]string) CEFEvent {
	event := NewNsgCEFEvent()
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
		time.Now().Format(CEFTimeFormat), "hostname", "CEF")
	assert.Equal(t, expected, out, "Base CEF Message should get formatted properly")
}

func TestCEFSyslogFormatterWithTime(t *testing.T) {
	out := CEFSyslogFormatter(syslog.LOG_ERR, "hostname", "tag", "Jan 02 15:04:05|CEF")
	expected := fmt.Sprintf("%s %s %s",
		"Jan 02 15:04:05", "hostname", "CEF")
	assert.Equal(t, expected, out, "CEF Formatter should use specified timestamp when provided.")
}

func TestToSyslogEvent(t *testing.T) {
	for _, tt := range testEvents {
		slText, err := tt.event.SyslogText()
		assert.Nil(t, err, "got error converting to syslog")
		expectedCEF := fmt.Sprintf(tt.expectedFormat, tt.event.Time.Format(CEFTimeFormat))
		assert.Equal(t, expectedCEF, slText, "should convert to CEF Properly")
	}
}

func TestMarshal(t *testing.T) {
	for _, tt := range testEvents {
		_, err := json.MarshalIndent(tt.event, "", "    ")
		assert.Nil(t, err, "no error")
	}
}
