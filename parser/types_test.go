package parser

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"time"
)

var (
	sampleName    = "resourceId=/SUBSCRIPTIONS/SUBI/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/RGNAME-NSG/y=2017/m=06/d=09/h=21/m=00/PT1H.json"
	sampleNsgFile = "../testdata/nsg_log_sample.json"
	sampleNsgLog  = NsgLog{}
	timeLayout    = "01/02 15:04:05 GMT 2006"
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

func TestUnmarshal(t *testing.T) {
	sort.Sort(sampleNsgLog.Records)
	assert.Equal(t, 40, len(sampleNsgLog.Records), "should unmarshal all 40 records")
}

func TestGetLogTimeFromName(t *testing.T) {
	logTime, err := getLogTimeFromName(sampleName)
	if err != nil {
		t.Error(err)
	}
	testTime, _ := time.Parse(timeLayout, "06/09 21:00:00 GMT 2017")
	assert.Equal(t, testTime, logTime, "Log Time Should be Extracted from Log File Name")
}

func TestGetRecordsAfter(t *testing.T) {
	testTime, _ := time.Parse(timeLayout, "06/09 20:33:00 GMT 2017")
	afterRecords := sampleNsgLog.Records.After(testTime)
	assert.Equal(t, 14, len(afterRecords), "should filter out older records")
}

func TestShortName(t *testing.T) {
	assert.Equal(t, "NSG-123", sampleNsgLog.ShortName(), "should filter out older records")
}
