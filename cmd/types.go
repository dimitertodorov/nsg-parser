package cmd

import "time"

type NsgLog struct {
	Records []struct {
		Time time.Time `json:"time"`
		SystemID string `json:"systemId"`
		Category string `json:"category"`
		ResourceID string `json:"resourceId"`
		OperationName string `json:"operationName"`
		Properties struct {
			Version int `json:"Version"`
			Flows []struct {
				Rule string `json:"rule"`
				Flows []struct {
					Mac string `json:"mac"`
					FlowTuples []string `json:"flowTuples"`
				} `json:"flows"`
			} `json:"flows"`
		} `json:"properties"`
	} `json:"records"`
}

type ArcSightLog struct {
	Time time.Time `json:"time"`
	SystemID string `json:"systemId"`
	Category string `json:"category"`
	ResourceID string `json:"resourceId"`
	OperationName string `json:"operationName"`
	Rule string `json:"rule"`
	Mac string `json:"mac"`
	SourceIp string `json:"sourceIp"`
	DestinationIp string `json:"destinationIp"`
	SourcePort string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Protocol string `json:"protocol"`
	TrafficFlow string `json:"trafficFlow"`
	Traffic string`json:"traffic"`
}