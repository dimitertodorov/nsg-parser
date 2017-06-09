package nsgsyslog

import (
	"bytes"
	"fmt"
	syslog "github.com/RackSec/srslog"
	log "github.com/Sirupsen/logrus"
	"github.com/dimitertodorov/nsg-parser/model"
	"text/template"
)

var (
	sysLog    *syslog.Writer
	logFormat = "{{.Timestamp}},{{.Rule}},{{.Mac}},{{.SourceIp}},{{.SourcePort}},{{.DestinationIp}},{{.DestinationPort}},{{.Protocol}},{{.TrafficFlow}},{{.Traffic}}"
)

func InitClient(protocol, host, port string) error{
	logger, err := syslog.Dial(protocol, fmt.Sprintf("%s:%s", host, port),
		syslog.LOG_ERR, "nsg-logs")
	if err != nil {
		log.Fatal(err)
		return err
	}
	logger.SetFormatter(syslog.RFC5424Formatter)
	sysLog = logger
	return nil
}

func SendEvent(flowLog model.NsgFlowLog) {
	var message bytes.Buffer
	logTemplate, err := template.New("nsgFlowTemplate").Parse(logFormat)
	if err != nil {
		log.Errorf("event_send_error %s", err)
		return
	}
	err = logTemplate.Execute(&message, flowLog)
	if err != nil {
		log.Errorf("event_format_error %s", err)
		return
	}
	fmt.Fprintf(sysLog, "%s", message.String())
}
