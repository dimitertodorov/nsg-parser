package cmd

import (
	"encoding/json"
	"github.com/dimitertodorov/nsg-parser/parser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"time"
)

//Useful command for testing your arcsight endpoint.
var testSendCmd = &cobra.Command{
	Use:   "test_send",
	Short: "Test Sending events to Syslog.",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initSyslog()
		logs := []byte(`[{
    "CEFVersion": 0,
    "DeviceVendor": "Microsoft",
    "DeviceProduct": "Azure NSG",
    "DeviceVersion": "1",
    "DeviceEventClassId": "nsg-flow",
    "Time": "2017-06-22T05:58:09.8052328-04:00",
    "Name": "nsg-flow",
    "Severity": 0,
    "Extension": {
        "cs1": "UserRule_HTTP",
        "deviceDirection": "0",
        "dmac": "00:0D:3A:F3:38:54",
        "dpt": "80",
        "dst": "10.44.160.4",
        "outcome": "Allow",
        "proto": "TCP",
        "spt": "15425",
        "src": "10.22.1.8",
        "start": "1498075171000"
    }}]`)
		events := []parser.CEFEvent{}
		err := json.Unmarshal(logs, &events)
		if err != nil {
			log.Fatal(err)
		}

		for _, flowLog := range events {
			flowLog.Time = time.Now().Add(-15 * time.Minute)
			syslogClient.SendEvent(flowLog)
		}
	},
}

func init() {
	processCmd.AddCommand(testSendCmd)
}
