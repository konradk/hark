package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"hark/internal/ai"
	"hark/internal/history"
	"hark/internal/hyprland"
	"hark/internal/ipc"
	"hark/internal/screenshot"
	"hark/internal/settings"
)

func (a *appState) ask(ctx context.Context, req ipc.Request, send func(any) error) error {
	var askReq ai.Request
	if err := decodeParams(req, "ask", &askReq); err != nil {
		return err
	}
	cfg := a.snapshotConfig()
	providers := a.snapshotProviders()
	if askReq.Model == "" {
		askReq.Model = cfg.Provider.DefaultModel
	}
	if askReq.ReasoningEffort == "" {
		askReq.ReasoningEffort = cfg.Provider.DefaultReasoningEffort
	}
	if askReq.ConversationID == "" {
		askReq.ConversationID = newConversationID()
	}
	if err := ai.ValidateRequest(askReq, allowedModelIDs(cfg)); err != nil {
		return fmt.Errorf("invalid ask request: %w", err)
	}
	if err := a.validateAttachments(askReq.Attachments); err != nil {
		return err
	}

	modelConfig, ok := configuredModel(cfg, askReq.Model)
	if !ok {
		return fmt.Errorf("model %q is not configured", askReq.Model)
	}
	providerName, _ := configuredProviderForModel(cfg, askReq.Model)
	if !settings.SupportsReasoningEffort(modelConfig.ReasoningEfforts, askReq.ReasoningEffort) {
		return fmt.Errorf("reasoning effort %q is not supported by model %q", askReq.ReasoningEffort, askReq.Model)
	}
	provider, ok := providers[providerName]
	if !ok {
		return fmt.Errorf("no provider configured for model %q", askReq.Model)
	}
	if state, ok := a.runtimeState(askReq.ConversationID); ok && state.Provider == providerName && state.Model == askReq.Model {
		askReq.ProviderState = append(askReq.ProviderState[:0], state.ProviderState...)
	}
	if statusProvider, ok := provider.(ai.InitialStatusProvider); ok {
		if status := statusProvider.InitialStatus(askReq); status != "" {
			if err := send(ai.Event{Type: ai.EventStatus, Provider: providerName, Text: status}); err != nil {
				return err
			}
		}
	}

	events, err := provider.Ask(ctx, askReq)
	if err != nil {
		return err
	}

	var answer ai.ResponseBuffer
	a.setLatestAnswer(askReq.ConversationID, "")
	for event := range events {
		if event.Provider == "" {
			event.Provider = providerName
		}
		if event.Type == ai.EventDelta {
			if err := answer.Append(event.Text); err != nil {
				return err
			}
			a.setLatestAnswer(askReq.ConversationID, answer.String())
		} else if event.Type == ai.EventFinal {
			if err := answer.Replace(event.Text); err != nil {
				return err
			}
			a.setLatestAnswer(askReq.ConversationID, answer.String())
		} else if event.Type == ai.EventDone {
			a.setProviderState(askReq.ConversationID, providerName, askReq.Model, event.ProviderState)
			if answer.Len() == 0 {
				if err := send(event); err != nil {
					return err
				}
				continue
			}
			var historyErr error
			saveHistory, err := a.saveHistoryEnabled(ctx)
			if err != nil {
				historyErr = fmt.Errorf("read save_history setting: %w", err)
			} else if saveHistory {
				_, historyErr = a.history.Add(ctx, history.Entry{
					ConversationID: askReq.ConversationID,
					Prompt:         askReq.Prompt,
					Response:       answer.String(),
					Provider:       providerName,
					Model:          askReq.Model,
					Attachments:    askReq.Attachments,
				})
			}
			if historyErr != nil {
				if a.logger != nil {
					a.logger.Printf("history save failed for %s: %v", askReq.ConversationID, historyErr)
				}
				if err := send(ai.Event{
					Type:  ai.EventWarning,
					Error: "Answer generated, but history could not be saved: " + historyErr.Error(),
				}); err != nil {
					return err
				}
			}
		}
		if err := send(event); err != nil {
			return err
		}
	}
	return nil
}

func (a *appState) validateAttachments(attachments []ai.Attachment) error {
	directory := a.attachmentDir
	if directory == "" {
		directory = screenshot.DefaultDir()
	}
	for index, attachment := range attachments {
		if !screenshot.ManagedPath(directory, attachment.Path) {
			return fmt.Errorf("attachments[%d].path must be a Hark screenshot in %s", index, directory)
		}
	}
	return nil
}

func newConversationID() string {
	var random [12]byte
	if _, err := rand.Read(random[:]); err == nil {
		return "chat-" + hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("chat-%d", time.Now().UnixNano())
}

func (a *appState) copyLatestRequest(ctx context.Context, req ipc.Request) (any, error) {
	var request conversationRequest
	if err := decodeParams(req, "copy_latest", &request); err != nil {
		return nil, err
	}
	if err := validateStateID(request.ConversationID, "conversation_id"); err != nil {
		return nil, err
	}
	state, ok := a.runtimeState(request.ConversationID)
	if !ok || state.LatestAnswer == "" {
		return nil, fmt.Errorf("no answer is available for conversation %q", request.ConversationID)
	}
	if err := a.clip.Copy(ctx, state.LatestAnswer); err != nil {
		return nil, err
	}
	return map[string]bool{"copied": true}, nil
}

func (a *appState) copyText(ctx context.Context, req ipc.Request) (any, error) {
	var request copyTextRequest
	if err := decodeParams(req, "copy_text", &request); err != nil {
		return nil, err
	}
	if err := validateActionText(request.Text); err != nil {
		return nil, err
	}
	if err := a.clip.Copy(ctx, request.Text); err != nil {
		return nil, err
	}
	return map[string]bool{"copied": true}, nil
}

func (a *appState) pasteText(ctx context.Context, req ipc.Request) (any, error) {
	var request pasteTextRequest
	if err := decodeParams(req, "paste_text", &request); err != nil {
		return nil, err
	}
	if err := validateStateID(request.StateID, "state_id"); err != nil {
		return nil, err
	}
	if err := validateActionText(request.Text); err != nil {
		return nil, err
	}
	state, _ := a.runtimeState(request.StateID)
	return a.paste(ctx, request.Text, state.PreviousWindow)
}

func (a *appState) pasteLatest(ctx context.Context, req ipc.Request) (any, error) {
	var request conversationRequest
	if err := decodeParams(req, "paste_latest", &request); err != nil {
		return nil, err
	}
	if err := validateStateID(request.ConversationID, "conversation_id"); err != nil {
		return nil, err
	}
	state, ok := a.runtimeState(request.ConversationID)
	if !ok || state.LatestAnswer == "" {
		return nil, fmt.Errorf("no answer is available for conversation %q", request.ConversationID)
	}
	return a.paste(ctx, state.LatestAnswer, state.PreviousWindow)
}

func (a *appState) paste(ctx context.Context, text string, previous hyprland.Window) (any, error) {
	if err := a.clip.Copy(ctx, text); err != nil {
		return nil, err
	}
	if a.cfg.Paste.RestoreFocus && previous.Address != "" {
		if err := a.hypr.FocusWindow(ctx, previous); err != nil {
			return nil, err
		}
	}
	if err := a.paster.Paste(ctx); err != nil {
		return nil, err
	}
	return map[string]bool{"pasted": true}, nil
}

func (a *appState) rememberActiveWindow(ctx context.Context, req ipc.Request) (any, error) {
	var request stateRequest
	if err := decodeParams(req, "remember_active_window", &request); err != nil {
		return nil, err
	}
	if err := validateStateID(request.StateID, "state_id"); err != nil {
		return nil, err
	}
	window, err := a.hypr.ActiveWindow(ctx)
	if err != nil {
		return nil, err
	}
	a.setPreviousWindow(request.StateID, window)
	return window, nil
}

func (a *appState) activeWindow(ctx context.Context, req ipc.Request) (any, error) {
	if err := validateNoParams(req, "active_window"); err != nil {
		return nil, err
	}
	return a.hypr.ActiveWindow(ctx)
}

func (a *appState) screenshotRegion(ctx context.Context, req ipc.Request) (any, error) {
	if err := validateNoParams(req, "screenshot_region"); err != nil {
		return nil, err
	}
	return a.capturer.CaptureRegion(ctx)
}

func (a *appState) screenshotActiveWindow(ctx context.Context, req ipc.Request) (any, error) {
	if err := validateNoParams(req, "screenshot_active_window"); err != nil {
		return nil, err
	}
	window, err := a.hypr.ActiveWindow(ctx)
	if err != nil {
		return nil, err
	}
	return a.capturer.CaptureWindow(ctx, window.At[0], window.At[1], window.Size[0], window.Size[1])
}
