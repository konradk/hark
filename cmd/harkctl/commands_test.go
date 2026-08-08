package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"hark/internal/ipc"
)

func TestMultiFlagRejectsEmptyAndCollectsValues(t *testing.T) {
	t.Parallel()

	var values multiFlag
	if err := values.Set("  "); err == nil {
		t.Fatal("expected empty image path to be rejected")
	}
	if err := values.Set(" first.png "); err != nil {
		t.Fatalf("set first value: %v", err)
	}
	if err := values.Set("second.jpg"); err != nil {
		t.Fatalf("set second value: %v", err)
	}
	if got, want := values.String(), "first.png,second.jpg"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestParseIDArg(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		args    []string
		want    int64
		wantErr string
	}{
		{name: "valid", args: []string{"42"}, want: 42},
		{name: "missing", wantErr: "expected usage"},
		{name: "not a number", args: []string{"abc"}, wantErr: "parse id"},
		{name: "numeric prefix", args: []string{"42oops"}, wantErr: "parse id"},
		{name: "zero", args: []string{"0"}, wantErr: "positive"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseIDArg(test.args, "expected usage")
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("parseIDArg() error = %v", err)
				}
				if got != test.want {
					t.Fatalf("parseIDArg() = %d, want %d", got, test.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("parseIDArg() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func withStdin(t *testing.T, content string) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create stdin file: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write stdin file: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind stdin file: %v", err)
	}

	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = original
		_ = file.Close()
	})
}

func TestReadAskPayloadFromStdin(t *testing.T) {
	withStdin(t, `{"prompt":"  what is this  ","messages":[{"role":"user","content":"prior"}]}`)

	payload, err := readAskPayload(true, nil)
	if err != nil {
		t.Fatalf("readAskPayload returned error: %v", err)
	}
	if payload.Prompt != "what is this" {
		t.Fatalf("prompt = %q", payload.Prompt)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Content != "prior" {
		t.Fatalf("messages = %#v", payload.Messages)
	}
}

func TestReadAskPayloadRejectsPositionalPromptWithStdin(t *testing.T) {
	withStdin(t, `{"prompt":"from stdin"}`)

	if _, err := readAskPayload(true, []string{"also", "positional"}); err == nil {
		t.Fatal("expected --stdin with a positional prompt to be rejected")
	}
}

func TestReadAskPayloadRejectsUnknownFields(t *testing.T) {
	withStdin(t, `{"prompt":"hello","api_key":"leak"}`)

	if _, err := readAskPayload(true, nil); err == nil {
		t.Fatal("expected unknown stdin fields to be rejected")
	}
}

func TestReadAskPayloadRequiresStdin(t *testing.T) {
	if _, err := readAskPayload(false, []string{"private prompt"}); err == nil {
		t.Fatal("readAskPayload accepted a positional prompt")
	}
}

func TestReadAskPayloadRejectsTrailingJSON(t *testing.T) {
	withStdin(t, `{"prompt":"hello"} {"prompt":"hidden"}`)
	if _, err := readAskPayload(true, nil); err == nil {
		t.Fatal("readAskPayload accepted multiple JSON values")
	}
}

func TestReadTextInputPreservesWhitespaceAndBoundsInput(t *testing.T) {
	withStdin(t, "  indented\n")
	text, err := readTextInput(true, nil)
	if err != nil {
		t.Fatalf("readTextInput returned error: %v", err)
	}
	if text != "  indented" {
		t.Fatalf("text = %q", text)
	}
}

func TestReadTextInputRejectsProcessArguments(t *testing.T) {
	if _, err := readTextInput(true, []string{"private text"}); err == nil {
		t.Fatal("readTextInput accepted process arguments")
	}
}

func TestReadTextInputRejectsOversizedInput(t *testing.T) {
	withStdin(t, strings.Repeat("x", ipc.MaxTextActionBytes+1))
	if _, err := readTextInput(true, nil); err == nil {
		t.Fatal("readTextInput accepted oversized input")
	}
}

func TestReadSecretValueRejectsOversizedInput(t *testing.T) {
	withStdin(t, strings.Repeat("x", maxSecretBytes+1))
	if _, err := readSecretValue(true); err == nil {
		t.Fatal("readSecretValue accepted oversized input")
	}
}
