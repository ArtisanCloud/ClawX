package autonomy

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		class      FailureClass
		recover    bool
		reasonPart string
	}{
		{name: "nil", err: nil, class: FailureClassUnknown, recover: false, reasonPart: "no_error"},
		{name: "permission", err: errors.New("execute request: cwd is outside allowed roots"), class: FailureClassPermission, recover: false, reasonPart: "permission"},
		{name: "auth", err: errors.New("401 unauthorized"), class: FailureClassAuth, recover: false, reasonPart: "auth"},
		{name: "network", err: errors.New("dial tcp: no such host"), class: FailureClassNetwork, recover: true, reasonPart: "network"},
		{name: "resource", err: context.DeadlineExceeded, class: FailureClassResource, recover: true, reasonPart: "deadline"},
		{name: "resource-killed", err: errors.New("execute request: run codex cli: signal: killed"), class: FailureClassResource, recover: true, reasonPart: "resource"},
		{name: "tool", err: errors.New("exit status 1"), class: FailureClassTool, recover: true, reasonPart: "tool"},
		{name: "unknown", err: errors.New("something odd happened"), class: FailureClassUnknown, recover: false, reasonPart: "unclassified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFailure(tc.err)
			if got.Class != tc.class {
				t.Fatalf("class mismatch: got=%s want=%s", got.Class, tc.class)
			}
			if got.Recoverable != tc.recover {
				t.Fatalf("recoverable mismatch: got=%v want=%v", got.Recoverable, tc.recover)
			}
			if got.Reason == "" {
				t.Fatalf("reason should not be empty")
			}
		})
	}
}
