package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"hark/internal/ai"
	"hark/internal/ipc"
	"hark/internal/settings"
)

// maxAskPayloadBytes matches the daemon's IPC request limit; larger bodies are
// rejected there anyway.
const maxAskPayloadBytes = 1 << 20

// askPayload carries prompt text and prior turns over stdin so they never reach
// the world-readable process arguments.
type askPayload struct {
	Prompt   string       `json:"prompt"`
	Messages []ai.Message `json:"messages,omitempty"`
}

func ask(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("ask", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write newline-delimited JSON events")
	fromStdin := flags.Bool("stdin", false, "read a JSON body with prompt and prior messages from stdin")
	model := flags.String("model", "", "model override")
	reasoningEffort := flags.String("reasoning-effort", "", "reasoning effort override")
	conversationID := flags.String("conversation-id", "", "conversation identifier for grouping history")
	images := multiFlag{}
	flags.Var(&images, "image", "image attachment path; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}

	payload, err := readAskPayload(*fromStdin, flags.Args())
	if err != nil {
		return err
	}
	if payload.Prompt == "" {
		return errors.New("usage: harkctl ask [--json] [--model model] [--conversation-id id] --stdin")
	}
	attachments := make([]ai.Attachment, 0, len(images))
	for _, path := range images {
		attachments = append(attachments, ai.Attachment{Type: "image", Path: path})
	}

	var wroteText bool
	err = ipc.CallStream(ctx, socketPath, "ask", ai.Request{
		ConversationID:  *conversationID,
		Prompt:          payload.Prompt,
		Model:           *model,
		ReasoningEffort: *reasoningEffort,
		Messages:        payload.Messages,
		Attachments:     attachments,
	}, func(raw json.RawMessage) error {
		var transport struct {
			OK    *bool  `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &transport); err == nil && transport.OK != nil {
			if !*transport.OK {
				if transport.Error == "" {
					return errors.New("request failed")
				}
				return errors.New(transport.Error)
			}
			return nil
		}

		var event ai.Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return fmt.Errorf("decode ask event: %w", err)
		}
		if *jsonOutput {
			fmt.Println(string(raw))
			return nil
		}
		switch event.Type {
		case ai.EventDelta:
			wroteText = true
			fmt.Print(event.Text)
		case ai.EventFinal:
			if !wroteText {
				wroteText = true
				fmt.Print(event.Text)
			} else if len(event.Sources) > 0 {
				fmt.Print("\n\nSources:\n")
				for index, source := range event.Sources {
					if source.Title != "" {
						fmt.Printf("%d. %s — %s\n", index+1, source.Title, source.URL)
					} else {
						fmt.Printf("%d. %s\n", index+1, source.URL)
					}
				}
			}
		case ai.EventError:
			if event.Error == "" {
				return errors.New("provider request failed")
			}
			return errors.New(event.Error)
		case ai.EventWarning:
			if event.Error != "" {
				fmt.Fprintf(os.Stderr, "harkctl: warning: %s\n", event.Error)
			}
		case ai.EventDone:
			if wroteText {
				fmt.Println()
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ask failed: %w", err)
	}
	return nil
}

func readAskPayload(fromStdin bool, args []string) (askPayload, error) {
	if len(args) != 0 {
		return askPayload{}, errors.New("harkctl ask does not accept prompts in process arguments; use --stdin")
	}
	if !fromStdin {
		return askPayload{}, errors.New("harkctl ask requires --stdin so prompts do not appear in process arguments")
	}

	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxAskPayloadBytes+1))
	if err != nil {
		return askPayload{}, fmt.Errorf("read ask request from stdin: %w", err)
	}
	if len(data) > maxAskPayloadBytes {
		return askPayload{}, fmt.Errorf("ask request exceeds %d bytes", maxAskPayloadBytes)
	}

	var payload askPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return askPayload{}, fmt.Errorf("decode ask request from stdin: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return askPayload{}, errors.New("decode ask request from stdin: multiple JSON values")
		}
		return askPayload{}, fmt.Errorf("decode ask request from stdin: %w", err)
	}
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	return payload, nil
}

func models(ctx context.Context, socketPath string, args []string) error {
	return printIPCList(ctx, socketPath, "models", "models_list", args, &[]any{})
}

func reasoningModes(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("reasoning-modes", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	model := flags.String("model", "", "model whose supported modes should be listed")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl reasoning-modes [--json] [--model ID]")
	}
	modes := []settings.ReasoningMode{}
	params := any(nil)
	if *model != "" {
		params = map[string]string{"model": *model}
	}
	if err := ipc.Call(ctx, socketPath, "reasoning_modes_list", params, &modes); err != nil {
		return fmt.Errorf("reasoning modes: %w", err)
	}
	var (
		encoded []byte
		err     error
	)
	if *compactJSON {
		encoded, err = json.Marshal(modes)
	} else {
		encoded, err = json.MarshalIndent(modes, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("format reasoning modes: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func printIPCList(ctx context.Context, socketPath, command, method string, args []string, result any) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: harkctl %s [--json]", command)
	}
	if err := ipc.Call(ctx, socketPath, method, nil, result); err != nil {
		return fmt.Errorf("%s: %w", strings.ReplaceAll(command, "-", " "), err)
	}
	var (
		encoded []byte
		err     error
	)
	if *compactJSON {
		encoded, err = json.Marshal(result)
	} else {
		encoded, err = json.MarshalIndent(result, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("format %s: %w", strings.ReplaceAll(command, "-", " "), err)
	}
	fmt.Println(string(encoded))
	return nil
}

func theme(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("theme", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl theme [--json]")
	}
	var response ipc.Theme
	if err := ipc.Call(ctx, socketPath, "theme_get", nil, &response); err != nil {
		return fmt.Errorf("theme: %w", err)
	}
	var (
		encoded []byte
		err     error
	)
	if *compactJSON {
		encoded, err = json.Marshal(response)
	} else {
		encoded, err = json.MarshalIndent(response, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("format theme: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

type multiFlag []string

func (flagValues *multiFlag) String() string {
	return strings.Join(*flagValues, ",")
}

func (flagValues *multiFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("image path must not be empty")
	}
	*flagValues = append(*flagValues, value)
	return nil
}
