package parser

import (
	"bytes"
	log "github.com/sirupsen/logrus"
)

type NsgParserClient interface {
	ProcessNsgLogFile(*AzureNsgLogFile, chan AzureNsgLogFile) error
}

// Parses Blob.Name (Path) or Resource ID for NSG Name
func getNsgName(name string) (string, error) {
	nameTokens := NsgFileRegExp.FindStringSubmatch(name)

	if len(nameTokens) != 7 {
		log.Errorf("%d %s", len(nameTokens), name)
		return "", errResourceIdName
	}
	return nameTokens[1], nil
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
