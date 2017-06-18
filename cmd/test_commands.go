package cmd

import (
	"encoding/json"
	"github.com/dimitertodorov/nsg-parser/parser"
	"github.com/spf13/cobra"
)

//Useful command for testing your arcsight endpoint.
var testSendCmd = &cobra.Command{
	Use:   "test_send",
	Short: "Test Sending events to Syslog.",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initSyslog()
		logs := []byte(`[{
    "time": 1497477570,
    "systemId": "",
    "category": "",
    "resourceId": "/SUBSCRIPTIONS/RGNAME/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG",
    "operationName": "",
    "rule": "Fake_UDP_RULE",
    "mac": "00:0D:3A:F3:38:54",
    "sourceIp": "10.193.60.4",
    "destinationIp": "10.44.55.66",
    "sourcePort": "14953",
    "destinationPort": "80",
    "protocol": "U",
    "trafficFlow": "O",
    "traffic": "D"
  },
  {
    "time": 1497477572,
    "systemId": "",
    "category": "",
    "resourceId": "/SUBSCRIPTIONS/RGNAME/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG",
    "operationName": "",
    "rule": "Fake_TCP_RULE",
    "mac": "00:0D:3A:F3:38:54",
    "sourceIp": "10.44.55.66",
    "destinationIp": "10.193.160.4",
    "sourcePort": "14954",
    "destinationPort": "80",
    "protocol": "T",
    "trafficFlow": "I",
    "traffic": "A"
  }]`)
		aLogs := []parser.NsgFlowLog{}
		_ = json.Unmarshal(logs, &aLogs)
		for _, flowLog := range aLogs {
			syslogClient.SendEvent(flowLog)
		}
	},
}

func init() {
	processCmd.AddCommand(testSendCmd)
}
