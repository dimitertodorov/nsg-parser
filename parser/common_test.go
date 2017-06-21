package parser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	sampleName    = "resourceId=/SUBSCRIPTIONS/SUBI/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG/y=2017/m=06/d=09/h=21/m=00/PT1H.json"
	sampleNsgFile = "../testdata/nsg_log_sample.json"
	sampleNsgLog  = NsgLog{}

	sampleProcessStatusFile = "../testdata/process_status_sample.json"

	timeLayout = "01/02 15:04:05 GMT 2006"
)

func init() {
	file, err := ioutil.ReadFile(sampleNsgFile)
	if err != nil {
		fmt.Printf(fmt.Sprintf("Error Loading Sample Data: %s", err))
		os.Exit(1)
	}
	err = json.Unmarshal(file, &sampleNsgLog)
	if err != nil {
		fmt.Printf(fmt.Sprintf("Error Umarshalling Sample Data: %s", err))
		os.Exit(1)
	}
}
