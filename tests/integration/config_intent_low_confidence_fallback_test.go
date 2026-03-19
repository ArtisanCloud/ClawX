package integration

import "testing"

func TestConfigIntentLowConfidenceFallbackIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandLowConfidenceSuggestOnly",
		"TestHandleConfigChatCommandNaturalLanguageNonConfigFallsBack",
	)
}

