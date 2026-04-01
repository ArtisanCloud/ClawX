package autonomy

import (
	"fmt"
	"strings"
)

func EnforceRoutingIronLaw(messageText string, decisionKind string) error {
	text := strings.TrimSpace(messageText)
	decisionKind = strings.TrimSpace(strings.ToLower(decisionKind))
	isCommand := strings.HasPrefix(text, "/")

	if isCommand && decisionKind != "control" {
		return fmt.Errorf("routing iron law violation: slash command must route to control (kind=%s)", decisionKind)
	}
	if !isCommand && decisionKind == "control" {
		return fmt.Errorf("routing iron law violation: non-slash message must not route directly to control")
	}
	return nil
}
