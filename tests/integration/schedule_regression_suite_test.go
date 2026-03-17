package integration

import "testing"

func TestScheduleRegressionSuite(t *testing.T) {
	t.Run("flow", TestScheduleCommandFlow)
	t.Run("nl", TestScheduleNaturalLanguageRouting)
	t.Run("scope", TestScheduleScopeProjectIsolation)
}
