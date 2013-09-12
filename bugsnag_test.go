package bugsnag

import (
	"fmt"
	"testing"
	"time"
)

const testApiKey = "066f5ad3590596f9aa8d601ea89af845"

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

func TestMultipleNotify(t *testing.T) {
	client := NewNotifier(testApiKey)
	for i := 0; i < 3; i++ {
		func() {
			// calls recover() on panic, and doesn't call panic() again. panic() is swallowed.
			defer client.NotifyOnPanic(true)
			if i < 2 {
				panic(fmt.Errorf("Test Message One")) // Happens Twice
			} else {
				panic(fmt.Errorf("Test Message Two")) // Happens Once
			}
		}()
	}
	spinCount := 0
	for spinCount < 5 && client.SentNotificationCount() < 3 {
		time.Sleep(time.Second)
		spinCount++
	}
	if client.SentNotificationCount() < 3 {
		t.Error("Failed to send all notifications successfully")
	}
}
