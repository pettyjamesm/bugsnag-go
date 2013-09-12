package bugsnag

import (
	"testing"
)

func TestReleaseStage(t *testing.T) {
	client := NewNotifier("non-empty key value")
	client.SetReleaseStage("development")
	if client.WillNotify() {
		t.Error("Failed to recognize non-notifying stage.")
	}
	client.SetNotifyStages([]string{"development", "staging", "production"})
	if !client.WillNotify() {
		t.Error("Failed to recognize notifying release stage")
	}
}

func TestEmptyApiKey(t *testing.T) {
	client := NewNotifier("")
	if client.WillNotify() {
		t.Error("Failed to recognize non-notifying api key")
	}
	client.SetReleaseStage("development")
	if client.WillNotify() {
		t.Error("Failed to recognize non-notifying api key")
	}
	client.SetNotifyStages([]string{"development", "staging", "production"})
	if client.WillNotify() {
		t.Error("Failed to recognize non-notifying api key")
	}
}
