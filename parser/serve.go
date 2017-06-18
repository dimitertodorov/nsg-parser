package parser

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/common/version"
	"net/http"
	"runtime"
)

var (
	httpStatusClient *AzureClient
)

type NsgParserStatus struct {
	GoVersion          string
	Version            string
	ProcessStatus      *ProcessStatus
	BuildDate          string
	BuildUser          string
	Revision           string
	ProcessedFlowCount int64
}

func ServeClient(client *AzureClient, ip string, port int) {
	httpStatusClient = client
	http.HandleFunc("/status", GetProcessStatus)

	http.ListenAndServe(fmt.Sprintf("%s:%d", ip, port), nil)
}

func loadStatus() (NsgParserStatus, error) {
	processStatus, err := ReadProcessStatus(httpStatusClient.DataPath, "nsg-parser-status-file.json")

	if err != nil {
		return NsgParserStatus{}, err
	}

	nsgParserStatus := NsgParserStatus{
		GoVersion:          runtime.Version(),
		Version:            version.Version,
		BuildDate:          version.BuildDate,
		BuildUser:          version.BuildUser,
		Revision:           version.Revision,
		ProcessStatus:      &processStatus,
		ProcessedFlowCount: processedFlowCount.Count(),
	}

	return nsgParserStatus, nil
}

func GetProcessStatus(w http.ResponseWriter, r *http.Request) {
	nsgParserStatus, err := loadStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(nsgParserStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
