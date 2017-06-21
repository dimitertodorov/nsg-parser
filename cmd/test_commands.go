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
    "CefVersion": 0,
    "DeviceVendor": "Microsoft",
    "DeviceProduct": "Azure NSG",
    "DeviceVersion": "1",
    "DeviceEventClassId": "nsg-flow",
    "Name": "nsg-flow",
    "Severity": 0,
    "Extension": {
        "cs1": "UserRule_HTTP",
        "deviceDirection": "0",
        "dmac": "00:0D:3A:F3:38:54",
        "dpt": "80",
        "dst": "10.193.160.4",
        "outcome": "Allow",
        "proto": "TCP",
        "spt": "15425",
        "src": "10.199.1.8"
    }]`)
		events := []parser.CefEvent{}
		_ = json.Unmarshal(logs, &events)
		for _, flowLog := range events {
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
