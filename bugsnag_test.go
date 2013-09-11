package bugsnag

import (
	"testing"
)

func TestReleaseStage(t *testing.T) {
	client := NewNotifier("non-empty key value")
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

func TestEmptyApiKey(t *testing.T) {
	client := NewNotifier("")
	hidden := client.(*restNotifier)
	if hidden.willNotify {
		t.Error("Failed to recognize non-notifying api key")
	}
	client.SetReleaseStage("development")
	if hidden.willNotify {
		t.Error("Failed to recognize non-notifying api key")
	}
	client.SetNotifyStages([]string{"development", "staging", "production"})
	if hidden.willNotify {
		t.Error("Failed to recognize non-notifying api key")
	}
}
