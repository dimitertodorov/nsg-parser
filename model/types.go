package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	NsgFileRegExp *regexp.Regexp
)

func init() {
	NsgFileRegExp = regexp.MustCompile(`.*\/(.*)\/y=([0-9]{4})\/m=([0-9]{2})\/d=([0-9]{2})\/h=([0-9]{2})\/m=([0-9]{2}).*`)
}

type NsgLogFile struct {
	Name          string        `json:name`
	Etag          string        `json:etag`
	LastModified  time.Time     `json:last_modified`
	LastProcessed time.Time     `json:last_processed`
	LastCount     int           `json:last_count`
	LogTime       time.Time     "json:log_time"
	Blob          *storage.Blob `json:"-"`
	NsgLog        *NsgLog       `json:"-"`
	NsgName       string        `json:nsg_name`
}

func NewNsgLogFile(blob *storage.Blob) (NsgLogFile, error) {
	nsgLogFile := NsgLogFile{}
	nsgLogFile.Blob = blob
	nsgLogFile.Name = blob.Name
	nsgLogFile.Etag = blob.Properties.Etag
	nsgLogFile.LastModified = time.Time(blob.Properties.LastModified)

	logTime, err := getLogTimeFromName(blob.Name)
	nsgLogFile.LogTime = logTime

	nsgName, err := getNsgName(blob.Name)
	nsgLogFile.NsgName = nsgName

	return nsgLogFile, err
}

func (logFile *NsgLogFile) SaveToPath(path string) error {
	bm := NsgFileRegExp.FindStringSubmatch(logFile.Blob.Name)
	fileName := "default.json"
	if len(bm) == 7 {
		fileName = fmt.Sprintf("nsgLog-%s-%s%s%s%s%s.json", bm[1], bm[2], bm[3], bm[4], bm[5], bm[6])
		fileName = fmt.Sprintf("%s-%s.json", fileName, logFile.LastModified.Format("2006-01-02-15-04-05"))
	} else {
		return fmt.Errorf("Error Parsing Blob.Name")
	}
	outJson, err := json.Marshal(logFile.NsgLog)
	if err != nil {
		return fmt.Errorf("error marshalling to disk")
	}

	path = fmt.Sprintf("%s/%s", path, fileName)
	log.Debugf("SaveToPath() - %s", path)
	err = ioutil.WriteFile(path, outJson, 0666)
	return nil
}

func (logFile *NsgLogFile) LoadBlob() error {
	log.Debugf("LoadBlob() for %s", logFile.Blob.Name)
	readCloser, err := logFile.Blob.Get(nil)
	nsgLog := NsgLog{}
	if err != nil {
		return fmt.Errorf("get blob failed: %v", err)
	}
	defer readCloser.Close()
	bytesRead, err := ioutil.ReadAll(readCloser)
	err = json.Unmarshal(bytesRead, &nsgLog)
	if err != nil {
		return fmt.Errorf("json parse body failed: %v - %v", err, logFile.Blob.Name)
	}
	logFile.NsgLog = &nsgLog
	return nil
}

func getLogTimeFromName(name string) (time.Time, error) {
	nameTokens := NsgFileRegExp.FindStringSubmatch(name)

	if len(nameTokens) != 7 {
		return time.Time{}, fmt.Errorf("Name did not match Pattern. Expected something like: %s\n", "resourceId=/SUBSCRIPTIONS/A8BB5151-C23C-4C2A-8043-B58C190C97A6/RESOURCEGROUPS/SDCCDEV01RGP01/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SDCCDEV01EMCOL01-NSG/y=2017/m=06/d=09/h=00/m=00/PT1H.json")
	}

	timeLayout := "01/02 15:04:05 GMT 2006"
	year := nameTokens[2]
	month := nameTokens[3]
	day := nameTokens[4]
	hour := nameTokens[5]
	minute := nameTokens[6]

	timeString := fmt.Sprintf("%s/%s %s:%s:00 GMT %s", month, day, hour, minute, year)

	return time.Parse(timeLayout, timeString)
}

func getNsgName(name string) (string, error) {
	nameTokens := NsgFileRegExp.FindStringSubmatch(name)

	if len(nameTokens) != 7 {
		return "", fmt.Errorf("Name did not match Pattern. Expected something like: %s\n", "resourceId=/SUBSCRIPTIONS/A8BB5151-C23C-4C2A-8043-B58C190C97A6/RESOURCEGROUPS/SDCCDEV01RGP01/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/SDCCDEV01EMCOL01-NSG/y=2017/m=06/d=09/h=00/m=00/PT1H.json")
	}

	return nameTokens[1], nil
}

type NsgLog struct {
	Records Records `json:"records"`
}
type Records []Record

func (slice Records) Len() int {
	return len(slice)
}

func (slice Records) Less(i, j int) bool {
	return slice[i].Time.Before(slice[j].Time)
}

func (slice Records) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (slice Records) After(afterTime time.Time) Records {
	var returnRecords Records
	for _, record := range slice {
		if record.Time.After(afterTime) {
			returnRecords = append(returnRecords, record)
		}
	}
	return returnRecords
}

type Record struct {
	Time          time.Time `json:"time"`
	SystemID      string    `json:"systemId"`
	Category      string    `json:"category"`
	ResourceID    string    `json:"resourceId"`
	OperationName string    `json:"operationName"`
	Properties    struct {
		Version int `json:"Version"`
		Flows   []struct {
			Rule  string `json:"rule"`
			Flows []struct {
				Mac        string   `json:"mac"`
				FlowTuples []string `json:"flowTuples"`
			} `json:"flows"`
		} `json:"flows"`
	} `json:"properties"`
}

type NsgFlowLog struct {
	Timestamp       int64  `json:"time"`
	SystemID        string `json:"systemId"`
	Category        string `json:"category"`
	ResourceID      string `json:"resourceId"`
	OperationName   string `json:"operationName"`
	Rule            string `json:"rule"`
	Mac             string `json:"mac"`
	SourceIp        string `json:"sourceIp"`
	DestinationIp   string `json:"destinationIp"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Protocol        string `json:"protocol"`
	TrafficFlow     string `json:"trafficFlow"`
	Traffic         string `json:"traffic"`
}

func (nsgLog *NsgLog) ConvertToNsgFlowLog() (*[]NsgFlowLog, error) {
	flowLogs := []NsgFlowLog{}
	for _, record := range nsgLog.Records {
		for _, flow := range record.Properties.Flows {
			for _, subFlow := range flow.Flows {
				for _, flowTuple := range subFlow.FlowTuples {
					alog := NsgFlowLog{}
					tuples := strings.Split(flowTuple, ",")
					epochTime, _ := strconv.ParseInt(tuples[0], 10, 64)
					alog.ResourceID = record.ResourceID
					alog.Timestamp = epochTime
					alog.SourceIp = tuples[1]
					alog.DestinationIp = tuples[2]
					alog.SourcePort = tuples[3]
					alog.DestinationPort = tuples[4]
					alog.Protocol = tuples[5]
					alog.TrafficFlow = tuples[6]
					alog.Traffic = tuples[7]
					alog.Rule = flow.Rule
					alog.Mac = formatMac(subFlow.Mac)
					flowLogs = append(flowLogs, alog)
				}
			}
		}
	}
	return &flowLogs, nil
}

func formatMac(s string) string {
	var buffer bytes.Buffer
	var n_1 = 1
	var l_1 = len(s) - 1
	for i, rune := range s {
		buffer.WriteRune(rune)
		if i%2 == n_1 && i != l_1 {
			buffer.WriteRune(':')
		}
	}
	return buffer.String()
}
