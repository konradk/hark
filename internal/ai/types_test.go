package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponseBufferEnforcesLimit(t *testing.T) {
	var buffer ResponseBuffer
	if err := buffer.Append(strings.Repeat("a", MaxResponseBytes)); err != nil {
		t.Fatalf("append response at limit: %v", err)
	}
	if err := buffer.Append("b"); err == nil {
		t.Fatal("expected append beyond response limit to fail")
	}
	if buffer.Len() != MaxResponseBytes {
		t.Fatalf("buffer length = %d, want %d", buffer.Len(), MaxResponseBytes)
	}
	if err := buffer.Replace(strings.Repeat("c", MaxResponseBytes+1)); err == nil {
		t.Fatal("expected replace beyond response limit to fail")
	}
}

func TestProviderStateNeverCrossesJSONBoundaries(t *testing.T) {
	state := json.RawMessage(`[{"type":"reasoning","encrypted_content":"secret"}]`)
	for name, value := range map[string]any{
		"request": Request{Prompt: "hello", ProviderState: state},
		"event":   Event{Type: EventDone, ProviderState: state},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if strings.Contains(string(encoded), "encrypted_content") || strings.Contains(string(encoded), "secret") {
			t.Fatalf("%s exposed provider state: %s", name, encoded)
		}
	}
}

func TestValidateRequestRejectsNonManagedImageFormats(t *testing.T) {
	request := Request{
		ConversationID:  "chat-test",
		Prompt:          "describe",
		Model:           "gpt-test",
		ReasoningEffort: "low",
		Attachments: []Attachment{{
			Type:     "image",
			Path:     "/cache/hark/screenshots/window-test.png",
			MIMEType: "image/jpeg",
		}},
	}
	if err := ValidateRequest(request, []string{"gpt-test"}); err == nil {
		t.Fatal("ValidateRequest accepted a non-PNG managed attachment")
	}
}
