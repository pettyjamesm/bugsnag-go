package bugsnag

import (
	"fmt"
	"testing"
	"time"
)

func TestReleaseStage(t *testing.T) {
	client := NewBugsnagNotifier("")
	hidden := client.(*restNotifier)
	client.SetReleaseStage("development")
	if hidden.willNotify {
		t.Error("Failed to recognize non-notifying stage.")
	}
	client.SetNotifyStages([]string{"development", "staging", "production"})
	if !hidden.willNotify {
		t.Error("Failed to recognize notifying release stage")
	}
}
