package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/dimitertodorov/nsg-parser/parser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
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

var testRangeCmd = &cobra.Command{
	Use:   "test_range",
	Short: "Test Getting Range",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		initSyslog()
		//bname := "resourceId=/SUBSCRIPTIONS/A8BB5151-C23C-4C2A-8043-B58C190C97A6/RESOURCEGROUPS/SDCCDEV01RGP01/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SDCCDEV01EMCOL01-NSG/y=2017/m=06/d=20/h=11/m=00/PT1H.json"
		log.Info(nsgAzureClient)
		beginTime := viper.GetString("begin_time")
		afterTime, _ := time.Parse(timeLayout, fmt.Sprintf("%s-00-00-GMT", beginTime))
		blobs, _, _ := nsgAzureClient.LoadUnprocessedBlobs(afterTime)
		//blobLength := len(*blobs)
		for _, b := range *blobs {
			getBlobRange(&b)
		}
	},
}

func getBlobRange(b *parser.NsgLogFile) {
	log.WithFields(log.Fields{
		"name": b.Blob.Name,
		"len":  b.Blob.Properties.ContentLength,
	}).Info("GOT IT")
	bOptions := storage.GetBlobRangeOptions{
		Range: &storage.BlobRange{uint64(208404), uint64(b.Blob.Properties.ContentLength)},
	}
	readCloser, err := b.Blob.GetRange(&bOptions)
	if err != nil {
		log.Fatalf("get blob range failed: %v", err)
	}
	defer readCloser.Close()
	bytesRead, err := ioutil.ReadAll(readCloser)
	log.Infof("----------------------------------------------------")
	formattedResults := fmt.Sprintf("{\"records\": [{ %s", string(bytesRead[:]))
	log.Info(formattedResults)
	nsgLog := parser.NsgLog{}
	err = json.Unmarshal([]byte(formattedResults), &nsgLog)
	if err != nil {
		log.Errorf("json parse body failed: %v - %v", err, b.Blob.Name)
	}
	b.NsgLog = &nsgLog
	log.Info(b.NsgLog)
}

func init() {
	processCmd.AddCommand(testRangeCmd)
}
