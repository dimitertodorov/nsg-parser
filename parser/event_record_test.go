package parser

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"fmt"
)

func TestLoadFiles(t *testing.T) {
	for _, tt := range fileTests {
		logs := loadTestFile(tt.testFile, t)
		assert.Equal(t, tt.expectedCount, len(logs.Records))
	}
}

func TestConvertToCEF(t *testing.T) {
	for testKey, tt := range fileTests {
		t.Run(fmt.Sprintf("%s", testKey), func(t *testing.T) {
			logs := loadTestFile(tt.testFile, t)
			events := []*CEFEvent{}
			for _, record := range logs.Records {
				cefEvents, errors := record.GetCEFList(GetCEFEventListOptions{})
				assert.Equal(t, 0, len(errors), "unexpected error during GetCEFList")
				events = append(events, cefEvents...)
			}
			assert.Equal(t, tt.expectedCEFEventCount, len(events))
		})
	}
}

func TestGetRecordsAfter(t *testing.T) {
	for testKey, tt := range fileTests {
		t.Run(fmt.Sprintf("%s", testKey), func(t *testing.T) {
			testTime, _ := time.Parse(timeLayout, tt.afterTime)
			logs := loadTestFile(tt.testFile, t)
			assert.Equal(t, tt.afterCount, len(logs.Records.After(testTime)))
		})
	}
}

func TestConvertToCEFError(t *testing.T) {
	for testKey, tt := range recordErrorTests {
		for testNumber, ttt := range tt {
			t.Run(fmt.Sprintf("%s_%d", testKey, testNumber), func(t *testing.T) {
				var record AzureNsgEventRecord
				err := json.Unmarshal(ttt.record, &record)
				if err != nil {
					t.Fatalf("got error loading record %s", err)
				}
				_, errors := record.GetCEFList(GetCEFEventListOptions{})
				assert.Equal(t, ttt.errorCount, len(errors))
				assert.EqualError(t, errors[0], ttt.firstErrorMessage)
			})

		}
	}
}

func TestGetSourceFileName(t *testing.T) {
	for _, tt := range miscRecordTests {
		for _, ttt := range tt {
			var record AzureNsgEventRecord
			err := json.Unmarshal(ttt.record, &record)
			assert.Nil(t, err, "got error loading record")
			assert.Equal(t, ttt.sourceFileName, record.getSourceFileName(), "filename did not match")
		}
	}
}

func TestGetSourceContainerName(t *testing.T) {
	for _, tt := range miscRecordTests {
		for _, ttt := range tt {
			var record AzureNsgEventRecord
			err := json.Unmarshal(ttt.record, &record)
			assert.Nil(t, err, "got error loading record")
			assert.Equal(t, ttt.sourceContainerName, record.getSourceContainerName(), "filename did not match")
		}
	}
}

func BenchmarkConvertToCEF(b *testing.B) {
	flowTest := fileTests["NetworkSecurityGroupFlowEvents"]
	t := testing.T{}
	logs := loadTestFile(flowTest.testFile, &t)
	for n := 0; n < b.N; n++ {
		events := []*CEFEvent{}
		for _, record := range logs.Records {
			cefEvents, _ := record.GetCEFList(GetCEFEventListOptions{})
			events = append(events, cefEvents...)
		}
	}
}
