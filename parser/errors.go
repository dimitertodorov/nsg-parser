package parser

import (
	"fmt"
)

var (
	errResourceIdName = fmt.Errorf("expected resourceId with name type /SUBSCRIPTIONS/SUBID/RESOURCEGROUPS/RGNAME/PROVIDERS/MICROSOFT.NETWORK/NETWORKSECURITYGROUPS/NSGNAME-NSG")
)
