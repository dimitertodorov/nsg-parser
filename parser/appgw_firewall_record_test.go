package parser

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)


func TestAppGwFirewallGetSourceFileName(t *testing.T) {
	for _, tt := range miscAppGwFirewallRecordTests {
		for _, ttt := range tt {
			var record AzureAppGwFirewallEventRecord
			err := json.Unmarshal(ttt.record, &record)
			assert.Nil(t, err, "got error loading record")
			assert.Equal(t, ttt.sourceFileName, record.getSourceFileName(), "filename did not match")
		}
	}
}

func TestAppGwFirewallGetSourceContainerName(t *testing.T) {
	for _, tt := range miscAppGwFirewallRecordTests {
		for _, ttt := range tt {
			var record AzureAppGwFirewallEventRecord
			err := json.Unmarshal(ttt.record, &record)
			assert.Nil(t, err, "got error loading record")
			assert.Equal(t, ttt.sourceContainerName, record.getSourceContainerName(), "filename did not match")
		}
	}
}

