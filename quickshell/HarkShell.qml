import QtQuick
import QtQuick.Controls
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import "components"

PanelWindow {
    id: root

    // Omarchy overrides these before initialization.
    property string hostMode: "standalone"
    property bool backendReady: hostMode === "standalone"
    property bool initialized: false
    signal openRequested(string payloadJson)
    signal closeRequested(bool animated)

    property bool previewAsking: false
    property string previewBackdropColor: "transparent"
    property var previewProviderConfiguredOverride: undefined
    property string screenshotSourceOverride: ""
    readonly property bool developerPreviewsEnabled: Quickshell.env("HARK_ENABLE_PREVIEWS") === "1"
    property bool asking: askProcess.running || previewAsking
    property string answerText: ""
    property string errorText: ""
    property string harkctlPath: Quickshell.env("HARKCTL_PATH") || "harkctl"
    readonly property string shellConfigPath: {
        const directory = String(Quickshell.shellDir);
        return directory + (directory.endsWith("/") ? "" : "/") + "shell.qml";
    }
    property string screenshotPath: ""
    property string screenshotSource: screenshotSourceOverride.length > 0 ? screenshotSourceOverride : screenshotPath.length > 0 ? "file://" + screenshotPath : ""
    property string conversationId: ""
    readonly property string clientStateId: "ui-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10)
    property string statusText: ""
    property string pendingCopyText: ""
    property string pendingPasteText: ""
    property string pendingAskPayload: ""
    property string askWarningText: ""
    property bool preferSystemTheme: true
    property string promptPlaceholder: hasAttachment ? "Ask about this screenshot..." : hasThread ? "Ask a follow-up..." : "Ask anything..."
    property var theme: ({
        "background": "transparent",
        "panel": "#15171e",
        "panel_border": "#2b303b",
        "panel_border_width": 1,
        "corner_radius": 14,
        "surface": "#11141a",
        "surface_elevated": "#1b1f28",
        "surface_hover": "#222733",
        "surface_active": "#2a3040",
        "input": "#0d1016",
        "button": "#1e232e",
        "button_disabled": "#171b23",
        "button_hover": "#262c39",
        "button_down": "#2e3543",
        "primary": "#a7c7ff",
        "primary_hover": "#b9d3ff",
        "primary_down": "#8fb6f2",
        "primary_text": "#0b1220",
        "text": "#d3d8e2",
        "text_strong": "#f2f4f8",
        "text_muted": "#8a93a3",
        "text_disabled": "#5b6373",
        "assistant": "#8fbce8",
        "error": "#e07878",
        "error_text": "#ffb4b4",
        "error_surface": "#251a1d",
        "error_border": "#5d3338",
        "selection_text": "#10131a"
    })
    property bool modelSelectorReady: false
    property bool reasoningSelectorReady: false
    property bool panelOpen: false
    property var threadMessages: []
    property var displayMessages: []
    property bool resultFollowBottom: true
    property bool resultPinning: false
    property int resumeGraceMs: 180000
    property double lastThreadActivityAt: 0
    property double lastDismissedAt: 0
    property string streamingAnswer: ""
    property var pendingAnswerChunks: []
    property bool hasThread: threadMessages.length > 0 || streamingAnswer.length > 0 || answerText.length > 0 || errorText.length > 0 || asking
    property bool hasAnswer: answerText.length > 0
    property bool hasAttachment: screenshotPath.length > 0 || screenshotProcess.running
    readonly property bool hasConfiguredProvider: developerPreviewsEnabled && previewProviderConfiguredOverride !== undefined ? Boolean(previewProviderConfiguredOverride) : settingsController.secretConfigured || settingsController.openRouterSecretConfigured
    property bool canSendPrompt: backendReady && hasConfiguredProvider && prompt.text.trim().length > 0 && !asking
    property bool promptEmpty: prompt.text.trim().length === 0
    // No prompt can be sent without a provider key, so the setup hint outranks
    // the recent list and stays up while typing. Otherwise a session that still
    // has history looks fully functional and silently drops every prompt.
    readonly property bool needsProviderSetup: !hasConfiguredProvider && !hasThread && !settingsController.secretsExpanded
    property bool showHistorySuggestions: settingsController.showRecentChats && promptEmpty && !settingsController.secretsExpanded && !hasThread && historyController.historyModel.count > 0 && !needsProviderSetup
    property bool showIdleHint: !hasThread && !settingsController.secretsExpanded && (needsProviderSetup || (promptEmpty && (!settingsController.showRecentChats || historyController.historyModel.count === 0)))
    property bool showResult: hasThread
    property bool showCopyAction: hasAnswer
    property bool showPasteAction: hasAnswer
    property bool showRetryAction: errorText.length > 0 && !asking && threadMessages.length > 0
    property bool showAskAction: asking || canSendPrompt || !hasAnswer
    property bool showAttachAction: screenshotProcess.running || screenshotPath.length === 0
    property int chromeHeight: 58 + 1 + (hasConfiguredProvider ? 1 + 44 : 0)
    property int contentSpacing: 10
    property int contentVPad: 24
    property bool compactPendingThread: asking && answerText.length === 0 && streamingAnswer.length === 0
    property int threadMinResultHeight: compactPendingThread ? 64 : 0
    property int threadMinPanelHeight: 210
    property int settingsPanelHeight: secretPanel.implicitHeight
    property int resultMaxHeight: 470
    property int panelMaxHeight: 620
    property int threadResultTargetHeight: Math.min(resultMaxHeight, Math.max(threadMinResultHeight, resultColumn.implicitHeight + 12))
    property int threadPanelTargetHeight: Math.min(panelMaxHeight, Math.max(threadMinPanelHeight, chromeHeight + contentVPad + threadResultTargetHeight + (hasAttachment ? 60 + contentSpacing : 0) + (showHistorySuggestions ? 132 + contentSpacing : 0) + (settingsController.secretsExpanded ? settingsPanelHeight + contentSpacing : 0)))
    property bool contentCollapsed: !hasThread && !hasAttachment && !settingsController.secretsExpanded && !showHistorySuggestions && !showIdleHint
    property int panelTargetHeight: hasThread ? threadPanelTargetHeight : settingsController.secretsExpanded ? chromeHeight + contentVPad + settingsPanelHeight + (hasAttachment ? 70 : 0) : hasAttachment ? 400 : showHistorySuggestions ? 430 : showIdleHint ? 320 : chromeHeight
    property int historyPanelTargetHeight: Math.max(160, contentArea.height - contentVPad - (hasAttachment ? 60 + contentSpacing : 0) - (settingsController.secretsExpanded ? 208 + contentSpacing : 0))
    readonly property int themeCornerRadius: Math.max(0, Number(theme.corner_radius ?? 14))
    readonly property int themePanelBorderWidth: Math.max(0, Number(theme.panel_border_width ?? 1))
    readonly property string themeFontFamily: String(theme.font_family ?? Qt.application.font.family)
    readonly property alias settings: settingsController
    readonly property alias history: historyController

    function cornerRadius(maximum) {
        return Math.min(themeCornerRadius, maximum);
    }

    function fontSize(role, fallback) {
        const value = Number(theme["font_" + role]);
        return isFinite(value) && value > 0 ? value : fallback;
    }

    SystemTheme {
        id: systemTheme

        onThemeChanged: {
            if (available && root.preferSystemTheme && theme.panel !== undefined && theme.text !== undefined)
                root.theme = theme;

        }
    }

    SettingsController {
        id: settingsController

        app: root
        harkctlPath: root.harkctlPath
        shortcutIntegration: root.hostMode === "omarchy" ? "omarchy" : "hyprland"
    }

    HistoryController {
        id: historyController

        app: root
        harkctlPath: root.harkctlPath
    }

    property Loader previewLoader: Loader {
        active: root.developerPreviewsEnabled
        source: "dev/DeveloperPreviews.qml"
        onLoaded: item.app = root
    }

    function sendPrompt() {
        if (!root.canSendPrompt)
            return ;

        const text = prompt.text.trim();
        const previousMessages = threadMessages.slice();
        streamRenderTimer.stop();
        pendingAnswerChunks = [];
        answerText = "";
        streamingAnswer = "";
        errorText = "";
        askWarningText = "";
        if (conversationId.length === 0)
            conversationId = createConversationId();

        // Prompt and prior turns go over stdin; process arguments are world-readable.
        let args = [harkctlPath, "ask", "--json", "--stdin"];
        args.push("--conversation-id", conversationId);
        if (settingsController.selectedModel.length > 0)
            args.push("--model", settingsController.selectedModel);

        if (settingsController.selectedReasoningEffort.length > 0)
            args.push("--reasoning-effort", settingsController.selectedReasoningEffort);

        if (screenshotPath.length > 0)
            args.push("--image", screenshotPath);

        pendingAskPayload = JSON.stringify({
            "prompt": text,
            "messages": previousMessages
        });
        resultFollowBottom = true;
        appendThreadMessage("user", text);
        renderThread("");
        prompt.text = "";
        askProcess.exec(args);
    }

    function createConversationId() {
        return "chat-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10);
    }

    function setDisplayMessages(messages) {
        displayMessages = messages;
    }

    function resultTargetContentY() {
        const flickable = resultScroll.contentItem;
        if (!flickable)
            return 0;

        // Pin the streaming tail while panel height animates.
        const resultRoom = Math.min(resultMaxHeight, flickable.contentHeight + 12) - flickable.height;
        const panelRoom = panelMaxHeight - panel.height;
        const pendingGrowth = Math.max(0, Math.min(resultRoom, panelRoom));
        return Math.max(0, flickable.contentHeight - flickable.height - pendingGrowth);
    }

    function resultIsAtBottom() {
        const flickable = resultScroll.contentItem;
        if (!flickable)
            return true;

        return flickable.contentY >= resultTargetContentY() - 4;
    }

    function scrollResultToBottom() {
        const flickable = resultScroll.contentItem;
        if (!flickable)
            return ;

        resultPinning = true;
        flickable.contentY = resultTargetContentY();
        resultPinning = false;
    }

    function keepResultAtBottom() {
        if (resultFollowBottom)
            scrollResultToBottom();

    }

    function appendThreadMessage(role, content) {
        const trimmed = String(content ?? "").trim();
        if (trimmed.length === 0)
            return ;

        const next = threadMessages.slice();
        next.push({
            "role": role,
            "content": trimmed
        });
        threadMessages = next;
        lastThreadActivityAt = Date.now();
    }

    function renderThread(pendingAssistantText) {
        const messages = [];
        for (const message of threadMessages) {
            messages.push({
                "role": message.role,
                "content": String(message.content ?? ""),
                "pending": false
            });
        }
        if (pendingAssistantText !== undefined && pendingAssistantText !== null)
            messages.push({
            "role": "assistant",
            "content": String(pendingAssistantText),
            "pending": true
        });

        setDisplayMessages(messages);
    }

    function queueAnswerDelta(text) {
        const delta = String(text ?? "");
        if (delta.length === 0)
            return;

        pendingAnswerChunks.push(delta);
        if (!streamRenderTimer.running)
            streamRenderTimer.start();
    }

    function flushAnswerDeltas() {
        streamRenderTimer.stop();
        if (pendingAnswerChunks.length > 0) {
            answerText += pendingAnswerChunks.join("");
            pendingAnswerChunks = [];
        }
        streamingAnswer = answerText;
        renderThread(streamingAnswer);
    }

    function discardAnswerDeltas() {
        streamRenderTimer.stop();
        pendingAnswerChunks = [];
    }

    function renderErrorMessage(text) {
        const messages = [];
        for (const message of threadMessages) {
            messages.push({
                "role": message.role,
                "content": String(message.content ?? ""),
                "pending": false
            });
        }
        messages.push({
            "role": "error",
            "content": String(text ?? "Request failed"),
            "pending": false
        });
        setDisplayMessages(messages);
    }

    function finishAssistantMessage(text) {
        const content = String(text ?? "").trim();
        if (content.length === 0)
            return ;

        appendThreadMessage("assistant", content);
        streamingAnswer = "";
        renderThread();
    }

    function applyRestoredConversation(newConversationId, messages, restoredAnswer, attachmentPath) {
        prompt.text = "";
        conversationId = newConversationId;
        threadMessages = messages;
        answerText = restoredAnswer;
        streamingAnswer = "";
        errorText = "";
        screenshotPath = attachmentPath;
        resultFollowBottom = true;
        renderThread();
    }

    function clearAttachment() {
        screenshotPath = "";
        attachmentStatus.text = "";
    }

    function focusSecretPanelInput() {
        secretPanel.focusSecretInput();
    }

    function beginSecretPanelShortcutRecording(action) {
        secretPanel.beginShortcutRecording(action);
    }

    function syncRetentionCombo() {
        secretPanel.syncRetention();
    }

    function initialize() {
        if (initialized || !backendReady)
            return;

        initialized = true;
        loadTheme();
        settingsController.loadDoctor();
        settingsController.loadModels();
        settingsController.loadSelectedModel();
        settingsController.loadSelectedReasoningEffort();
        settingsController.loadShowRecentChats();
        settingsController.loadSaveHistory();
        settingsController.loadHistoryRetention();
        settingsController.loadSecretStatus();
        settingsController.loadGlobalShortcut();
        historyController.loadHistory();
    }

    function openFromHost(payloadJson) {
        let payload = {};
        try {
            payload = payloadJson ? JSON.parse(payloadJson) : {};
        } catch (error) {
            payload = {};
        }

        if (String(payload.action ?? "") === "captureActiveWindow") {
            captureActiveWindowScreenshot();
            return;
        }
        showOverlay();
    }

    function closeFromHost(animated) {
        hideOverlay(animated !== false);
    }

    function requestOpen(payloadJson) {
        if (hostMode === "omarchy")
            openRequested(payloadJson || "");
        else
            openFromHost(payloadJson || "");
    }

    function requestClose(animated) {
        if (hostMode === "omarchy")
            closeRequested(animated !== false);
        else
            hideOverlay(animated !== false);
    }

    function revealPreview() {
        closeTimer.stop();
        root.visible = true;
        root.panelOpen = true;
        prompt.forceActiveFocus();
        resultFollowBottom = true;
        Qt.callLater(scrollResultToBottom);
    }

    function resetForPreview() {
        if (askProcess.running)
            askProcess.running = false;

        previewAsking = false;
        previewBackdropColor = "transparent";
        previewProviderConfiguredOverride = undefined;
        screenshotSourceOverride = "";
        preferSystemTheme = true;
        if (systemTheme.available)
            theme = systemTheme.theme;
        else
            loadTheme();
        settingsController.secretKeyInput = "";
        settingsController.openRouterSecretKeyInput = "";
        settingsController.secretsExpanded = false;
        historyController.confirmClearHistory = false;
        displayMessages = [];
        clearConversation();
    }

    function showDeveloperPreview(name) {
        if (!developerPreviewsEnabled || !previewLoader.item)
            return;

        previewLoader.item.show(name);
    }

    function showOverlay() {
        closeTimer.stop();
        if (!backendReady) {
            root.statusText = "Starting Hark service…";
            root.visible = true;
            root.panelOpen = true;
            return;
        }
        initialize();
        rememberWindowProcess.exec([harkctlPath, "remember-active-window", "--state-id", clientStateId]);
        settingsController.loadSelectedModel();
        settingsController.loadShowRecentChats();
        settingsController.loadSaveHistory();
        settingsController.loadHistoryRetention();
        settingsController.loadModels();
        settingsController.loadSecretStatus();
        historyController.loadHistory();
        if (threadIsStale())
            startFreshThread();

        lastDismissedAt = 0;
        root.visible = true;
        root.panelOpen = false;
        Qt.callLater(function() {
            root.panelOpen = true;
        });
        prompt.forceActiveFocus();
    }

    function promptForApiKeySetup() {
        if (settingsController.secretsExpanded)
            return ;

        historyController.historySelectionIndex = -1;
        settingsController.secretsExpanded = true;
        settingsController.loadGlobalShortcut();
        statusText = "Add an API key to start asking Hark";
        Qt.callLater(function() {
            secretPanel.focusSecretInput();
        });
    }

    function hideOverlay(animated) {
        closeTimer.stop();
        root.panelOpen = false;
        if (animated) {
            // Internal hides must not age the resumable thread.
            lastDismissedAt = Date.now();
            closeTimer.start();
        } else {
            root.visible = false;
        }
    }

    function handleAskEvent(line) {
        if (line.length === 0)
            return ;

        try {
            const event = JSON.parse(line);
            if (event.type === "status") {
                root.statusText = event.text ?? "";
                statusClearTimer.stop();
            } else if (event.type === "delta") {
                if (root.statusText === "Searching the web...")
                    root.statusText = "";

                queueAnswerDelta(event.text);
            } else if (event.type === "final") {
                discardAnswerDeltas();
                answerText = event.text ?? answerText;
                streamingAnswer = answerText;
                renderThread(streamingAnswer);
            } else if (event.type === "error") {
                discardAnswerDeltas();
                errorText = event.error ?? "Provider request failed";
                renderErrorMessage(errorText);
            } else if (event.type === "warning") {
                askWarningText = event.error ?? event.text ?? "The answer was generated with a warning.";
                root.statusText = askWarningText;
            } else if (event.type === "done") {
                flushAnswerDeltas();
                if (answerText.length === 0) {
                    root.statusText = askWarningText;
                    finishAssistantMessage("No response text returned.");
                    historyController.loadHistory();
                    return;
                }
                root.statusText = askWarningText;
                finishAssistantMessage(answerText);
                historyController.loadHistory();
            }
        } catch (error) {
            discardAnswerDeltas();
            errorText = String(error);
            renderErrorMessage("Could not parse ask response: " + errorText);
        }
    }

    function handleAskError(line) {
        if (line.length === 0)
            return ;

        discardAnswerDeltas();
        errorText = line;
        renderErrorMessage(line);
    }

    function retryLastPrompt() {
        if (askProcess.running || threadMessages.length === 0)
            return ;

        const next = threadMessages.slice();
        let promptText = "";
        for (let index = next.length - 1; index >= 0; index--) {
            if (next[index].role === "user") {
                promptText = String(next[index].content ?? "");
                next.splice(index, 1);
                break;
            }
        }
        if (promptText.length === 0)
            return ;

        threadMessages = next;
        errorText = "";
        answerText = "";
        streamingAnswer = "";
        prompt.text = promptText;
        sendPrompt();
    }

    function copyLatest() {
        if (answerText.length === 0)
            return ;

        copyMessageText(answerText);
    }

    function copyMessageText(text) {
        const value = String(text ?? "").trim();
        if (value.length === 0 || copyTextProcess.running)
            return ;

        pendingCopyText = value;
        root.statusText = "";
        copyTextProcess.exec([harkctlPath, "copy-text", "--stdin"]);
    }

    function pasteLatest() {
        if (answerText.length === 0 || pasteProcess.running)
            return ;

        pendingPasteText = answerText;
        root.statusText = "";
        root.hideOverlay(false);
        pasteProcess.exec([harkctlPath, "paste-text", "--state-id", clientStateId, "--stdin"]);
    }

    function captureScreenshotRegion() {
        if (screenshotProcess.running)
            return ;

        attachmentStatus.text = "Select region...";
        root.hideOverlay(false);
        screenshotProcess.exec([harkctlPath, "screenshot-region", "--json"]);
    }

    function captureActiveWindowScreenshot() {
        if (screenshotProcess.running)
            return ;

        attachmentStatus.text = "Capturing active window...";
        root.hideOverlay(false);
        activeWindowCaptureTimer.restart();
    }

    function handleScreenshot(line) {
        if (line.length === 0)
            return ;

        try {
            const attachment = JSON.parse(line);
            screenshotPath = attachment.path ?? "";
            attachmentStatus.text = screenshotPath.length > 0 ? "Screenshot attached" : "Screenshot failed";
            root.visible = true;
            root.panelOpen = true;
            prompt.forceActiveFocus();
        } catch (error) {
            attachmentStatus.text = "Screenshot parse failed";
            root.visible = true;
            root.panelOpen = true;
            prompt.forceActiveFocus();
        }
    }

    function toggleReasoningMode() {
        if (!reasoningSelector.enabled)
            return ;

        const currentIndex = reasoningSelector.indexOfValue(settingsController.selectedReasoningEffort);
        const nextIndex = (currentIndex + 1) % settingsController.reasoningModesModel.count;
        settingsController.selectedReasoningEffort = settingsController.reasoningModesModel.get(nextIndex).effort;
        settingsController.saveSelectedReasoningEffort();
        syncReasoningCombo();
    }

    function openModelPicker() {
        if (!modelSelector.enabled)
            return ;

        modelSelector.forceActiveFocus();
        modelSelector.popup.open();
    }

    function toggleSettingsPanel() {
        settingsController.secretsExpanded = !settingsController.secretsExpanded;
        if (settingsController.secretsExpanded) {
            historyController.historySelectionIndex = -1;
            settingsController.loadSecretStatus();
            settingsController.loadGlobalShortcut();
            if (!settingsController.secretConfigured && !settingsController.openRouterSecretConfigured)
                Qt.callLater(function() {
                    secretPanel.focusSecretInput();
                });
        } else {
            settingsController.secretKeyInput = "";
            settingsController.openRouterSecretKeyInput = "";
            settingsController.shortcutRecording = false;
            settingsController.shortcutRecordingAction = "";
            prompt.forceActiveFocus();
        }
    }

    function usePromptSuggestion(text) {
        prompt.text = text;
        prompt.forceActiveFocus();
        prompt.cursorPosition = prompt.text.length;
    }

    function startFreshThread() {
        // Preserve the unsent prompt and attachment.
        conversationId = "";
        threadMessages = [];
        answerText = "";
        streamingAnswer = "";
        errorText = "";
        askWarningText = "";
        historyController.historySelectionIndex = -1;
        root.statusText = "";
        resultFollowBottom = true;
        renderThread();
    }

    function clearConversation() {
        startFreshThread();
        prompt.text = "";
        screenshotPath = "";
        prompt.forceActiveFocus();
    }

    function showRecentConversations() {
        if (asking)
            return ;

        settingsController.secretsExpanded = false;
        clearConversation();
        historyController.loadHistory();
    }

    function threadIsStale() {
        if (!hasThread || asking)
            return false;

        const lastTouched = Math.max(lastThreadActivityAt, lastDismissedAt);
        return lastTouched > 0 && Date.now() - lastTouched > resumeGraceMs;
    }

    function cancelOrClose() {
        if (modelSelector.popup.visible) {
            modelSelector.popup.close();
            prompt.forceActiveFocus();
            return ;
        }
        if (reasoningSelector.popup.visible) {
            reasoningSelector.popup.close();
            prompt.forceActiveFocus();
            return ;
        }
        if (settingsController.secretsExpanded) {
            settingsController.secretsExpanded = false;
            settingsController.secretKeyInput = "";
            settingsController.openRouterSecretKeyInput = "";
            settingsController.shortcutRecording = false;
            settingsController.shortcutRecordingAction = "";
            prompt.forceActiveFocus();
            return ;
        }
        if (askProcess.running) {
            stopRequest();
            return ;
        }
        root.requestClose(true);
    }

    function dismissOverlay() {
        if (settingsController.secretsExpanded) {
            settingsController.secretsExpanded = false;
            settingsController.secretKeyInput = "";
            settingsController.openRouterSecretKeyInput = "";
            settingsController.shortcutRecording = false;
            settingsController.shortcutRecordingAction = "";
        }
        root.requestClose(true);
    }

    function stopRequest() {
        if (previewAsking) {
            previewAsking = false;
            streamingAnswer = "";
            root.statusText = "Stopped";
            renderErrorMessage("Stopped before a response was returned.");
            return ;
        }
        if (!askProcess.running)
            return ;

        askProcess.running = false;
        flushAnswerDeltas();
        if (answerText.length > 0) {
            finishAssistantMessage(answerText);
            root.statusText = "Stopped";
        } else {
            streamingAnswer = "";
            root.statusText = "Stopped";
            renderErrorMessage("Stopped before a response was returned.");
        }
    }

    function loadTheme() {
        if (!themeProcess.running)
            themeProcess.exec([harkctlPath, "theme", "--json"]);

    }

    function syncModelCombo() {
        if (!modelSelectorReady)
            return ;

        const index = modelSelector.indexOfValue(settingsController.selectedModel);
        if (index >= 0)
            modelSelector.currentIndex = index;

    }

    function syncReasoningCombo() {
        if (!reasoningSelectorReady)
            return ;

        const index = reasoningSelector.indexOfValue(settingsController.selectedReasoningEffort);
        if (index >= 0)
            reasoningSelector.currentIndex = index;

    }

    function handleTheme(line) {
        if (line.length === 0)
            return ;

        try {
            const resolved = JSON.parse(line);
            root.preferSystemTheme = String(resolved.name ?? "system") === "system";
            if (root.preferSystemTheme && systemTheme.available) {
                root.theme = systemTheme.theme;
                return ;
            }

            const colors = resolved.colors ?? {
            };
            root.theme = Object.assign({
            }, root.theme, colors);
        } catch (error) {
            root.statusText = "Theme parse failed";
        }
    }

    onStatusTextChanged: {
        if (statusText.length > 0 && !asking && errorText.length === 0)
            statusClearTimer.restart();
        else
            statusClearTimer.stop();
    }
    onAskingChanged: {
        if (!asking && statusText.length > 0 && errorText.length === 0)
            statusClearTimer.restart();

    }
    onBackendReadyChanged: {
        if (backendReady) {
            initialize();
            if (root.statusText === "Starting Hark service…")
                root.statusText = "";
        }
    }
    implicitWidth: 760
    implicitHeight: 560
    color: root.developerPreviewsEnabled && root.previewBackdropColor !== "transparent" ? root.previewBackdropColor : root.theme.background
    visible: false
    aboveWindows: true
    focusable: true
    exclusionMode: ExclusionMode.Ignore
    Component.onCompleted: Qt.callLater(root.initialize)

    anchors {
        left: true
        right: true
        top: true
        bottom: true
    }

    IpcHandler {
        function toggle() {
            if (root.visible)
                root.requestClose(true);
            else
                root.requestOpen("");
        }

        function open() {
            root.requestOpen("");
        }

        function close() {
            root.requestClose(true);
        }

        function captureActiveWindow() {
            if (root.hostMode === "omarchy")
                root.requestOpen(JSON.stringify({"action": "captureActiveWindow"}));
            else
                root.captureActiveWindowScreenshot();
        }

        function previewIdle() {
            root.showDeveloperPreview("idle");
        }

        function previewReset() {
            root.showDeveloperPreview("reset");
        }

        function previewThread() {
            root.showDeveloperPreview("thread");
        }

        function previewCode() {
            root.showDeveloperPreview("code");
        }

        function previewMarkdown() {
            root.showDeveloperPreview("markdown");
        }

        function previewTyping() {
            root.showDeveloperPreview("typing");
        }

        function previewStreaming() {
            root.showDeveloperPreview("streaming");
        }

        function previewSettings() {
            root.showDeveloperPreview("settings");
        }

        function previewHistory() {
            root.showDeveloperPreview("history");
        }

        function previewHistoryWithoutProvider() {
            root.showDeveloperPreview("history-without-provider");
        }

        function previewDemoHistory() {
            root.showDeveloperPreview("demo-history");
        }

        function previewDemoConversation() {
            root.showDeveloperPreview("demo-conversation");
        }

        function previewDemoAttachment() {
            root.showDeveloperPreview("demo-attachment");
        }

        function previewThemeTokyoNight() {
            root.showDeveloperPreview("theme-tokyo-night");
        }

        function previewThemeCatppuccinLatte() {
            root.showDeveloperPreview("theme-catppuccin-latte");
        }

        function previewThemeSolitude() {
            root.showDeveloperPreview("theme-solitude");
        }

        function previewThemeNord() {
            root.showDeveloperPreview("theme-nord");
        }

        target: "hark"
    }

    Shortcut {
        sequence: "Escape"
        onActivated: root.cancelOrClose()
    }

    Shortcut {
        sequence: "Ctrl+Return"
        onActivated: root.pasteLatest()
    }

    Shortcut {
        sequence: "Ctrl+Shift+C"
        onActivated: root.captureScreenshotRegion()
    }

    Shortcut {
        sequence: "Ctrl+N"
        onActivated: root.showRecentConversations()
    }

    Shortcut {
        sequence: "Ctrl+Backspace"
        onActivated: root.clearConversation()
    }

    Shortcut {
        sequence: "Ctrl+L"
        onActivated: {
            prompt.forceActiveFocus();
            prompt.selectAll();
        }
    }

    Shortcut {
        sequence: "Ctrl+M"
        onActivated: root.openModelPicker()
    }

    Shortcut {
        sequence: "Ctrl+T"
        onActivated: root.toggleReasoningMode()
    }

    Shortcut {
        sequence: "Ctrl+,"
        onActivated: root.toggleSettingsPanel()
    }

    Shortcut {
        sequence: "Ctrl+J"
        onActivated: historyController.moveHistorySelection(1)
    }

    Shortcut {
        sequence: "Ctrl+K"
        onActivated: {
            if (!historyController.moveHistorySelection(-1))
                root.clearConversation();

        }
    }

    Shortcut {
        sequence: "Ctrl+R"
        onActivated: historyController.loadHistory()
    }

    Timer {
        id: streamRenderTimer

        interval: 50
        repeat: false
        onTriggered: root.flushAnswerDeltas()
    }

    Timer {
        id: closeTimer

        interval: 130
        repeat: false
        onTriggered: root.visible = false
    }

    Timer {
        id: activeWindowCaptureTimer

        interval: 160
        repeat: false
        onTriggered: screenshotProcess.exec([harkctlPath, "screenshot-active-window", "--json"])
    }

    Timer {
        id: statusClearTimer

        interval: 3200
        repeat: false
        onTriggered: {
            if (!root.asking && root.errorText.length === 0)
                root.statusText = "";

        }
    }

    Process {
        id: askProcess

        stdinEnabled: true
        onStarted: {
            write(root.pendingAskPayload + "\n");
            stdinEnabled = false;
        }
        onExited: (exitCode, exitStatus) => {
            stdinEnabled = true;
            root.pendingAskPayload = "";
            if (exitCode !== 0 && root.errorText.length === 0) {
                root.errorText = "harkctl exited with code " + exitCode;
                root.renderErrorMessage(root.errorText);
            }
        }

        stdout: SplitParser {
            onRead: (line) => {
                return root.handleAskEvent(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                return root.handleAskError(line);
            }
        }

    }

    Process {
        id: rememberWindowProcess
    }

    Process {
        id: copyTextProcess

        stdinEnabled: true
        onStarted: {
            write(root.pendingCopyText + "\n");
            stdinEnabled = false;
        }
        onExited: (exitCode, exitStatus) => {
            stdinEnabled = true;
            root.pendingCopyText = "";
            if (exitCode === 0)
                root.statusText = "Copied message";
            else if (root.statusText.length === 0)
                root.statusText = "Copy failed";
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.statusText = line;

            }
        }

    }

    Process {
        id: pasteProcess

        stdinEnabled: true
        onStarted: {
            write(root.pendingPasteText + "\n");
            stdinEnabled = false;
        }
        onExited: (exitCode, exitStatus) => {
            stdinEnabled = true;
            root.pendingPasteText = "";
            if (exitCode !== 0 && root.statusText.length === 0) {
                root.visible = true;
                root.panelOpen = true;
                root.statusText = "Paste failed";
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0) {
                    root.visible = true;
                    root.panelOpen = true;
                    root.statusText = line;
                }
            }
        }

    }

    Process {
        id: screenshotProcess

        onExited: (exitCode, exitStatus) => {
            if (exitCode !== 0) {
                root.visible = true;
                root.panelOpen = true;
                prompt.forceActiveFocus();
                attachmentStatus.text = "Screenshot failed";
                if (root.statusText.length === 0)
                    root.statusText = "Screenshot failed";

            }
        }

        stdout: SplitParser {
            onRead: (line) => {
                return root.handleScreenshot(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0) {
                    attachmentStatus.text = line;
                    root.statusText = line.replace(/^harkctl:\s*/, "");
                }

            }
        }

    }

    Process {
        id: themeProcess

        stdout: SplitParser {
            onRead: (line) => {
                return root.handleTheme(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.statusText = line;

            }
        }

    }

    ListModel {
        id: retentionOptionsModel

        ListElement {
            label: "Forever"
            days: 0
        }

        ListElement {
            label: "7 days"
            days: 7
        }

        ListElement {
            label: "30 days"
            days: 30
        }

        ListElement {
            label: "90 days"
            days: 90
        }

        ListElement {
            label: "1 year"
            days: 365
        }

    }

    MouseArea {
        // Ignore drags that began inside the panel.
        id: backdrop

        property bool pressedOutside: false

        function outsidePanel(x, y) {
            const local = mapToItem(panel, x, y);
            return local.x < 0 || local.y < 0 || local.x > panel.width || local.y > panel.height;
        }

        anchors.fill: parent
        onPressed: (mouse) => {
            backdrop.pressedOutside = backdrop.outsidePanel(mouse.x, mouse.y);
        }
        onReleased: (mouse) => {
            if (backdrop.pressedOutside && backdrop.outsidePanel(mouse.x, mouse.y))
                root.dismissOverlay();

            backdrop.pressedOutside = false;
        }
    }

    RectangularShadow {
        anchors.fill: panel
        radius: panel.radius
        blur: 56
        spread: 0
        offset: Qt.vector2d(0, 10)
        color: Qt.rgba(0, 0, 0, 0.5)
        opacity: panel.opacity
        scale: panel.scale
        transformOrigin: Item.Center
    }

    Rectangle {
        id: panel

        width: Math.min(720, root.width - 32)
        height: Math.min(root.panelTargetHeight, root.height - 32)
        anchors.horizontalCenter: parent.horizontalCenter
        y: Math.round(Math.min(root.height * 0.2, root.height - height - 16))
        transformOrigin: Item.Center
        opacity: root.panelOpen ? 1 : 0
        scale: root.panelOpen ? 1 : 0.98
        radius: root.themeCornerRadius
        color: root.theme.panel
        border.color: root.theme.panel_border
        border.width: root.themePanelBorderWidth

        Column {
            anchors.fill: parent
            spacing: 0

            Item {
                width: parent.width
                height: 58

                TextInput {
                    id: prompt

                    anchors.left: parent.left
                    anchors.leftMargin: 20
                    anchors.right: recentChatsButton.visible ? recentChatsButton.left : escHint.left
                    anchors.rightMargin: 12
                    anchors.verticalCenter: parent.verticalCenter
                    height: 36
                    focus: true
                    color: root.theme.text_strong
                    selectedTextColor: root.theme.selection_text
                    selectionColor: root.theme.primary
                    font.family: root.themeFontFamily
                    font.pixelSize: root.fontSize("heading", 17)
                    verticalAlignment: TextInput.AlignVCenter
                    clip: true
                    Keys.onEscapePressed: root.cancelOrClose()
                    Keys.onPressed: (event) => {
                        const ctrl = event.modifiers & Qt.ControlModifier;
                        const shift = event.modifiers & Qt.ShiftModifier;
                        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && ctrl) {
                            event.accepted = true;
                            root.pasteLatest();
                        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                            event.accepted = true;
                            if (prompt.text.trim().length === 0 && historyController.openSelectedHistory())
                                return ;

                            root.sendPrompt();
                        } else if (event.key === Qt.Key_Down) {
                            event.accepted = historyController.moveHistorySelection(1);
                        } else if (event.key === Qt.Key_Up) {
                            event.accepted = historyController.moveHistorySelection(-1);
                        } else if (ctrl && shift && event.key === Qt.Key_C) {
                            event.accepted = true;
                            root.captureScreenshotRegion();
                        } else if (ctrl && event.key === Qt.Key_N) {
                            event.accepted = true;
                            root.showRecentConversations();
                        } else if (ctrl && event.key === Qt.Key_Backspace) {
                            event.accepted = true;
                            root.clearConversation();
                        } else if (ctrl && event.key === Qt.Key_L) {
                            event.accepted = true;
                            prompt.forceActiveFocus();
                            prompt.selectAll();
                        } else if (ctrl && event.key === Qt.Key_M) {
                            event.accepted = true;
                            root.openModelPicker();
                        } else if (ctrl && event.key === Qt.Key_T) {
                            event.accepted = true;
                            root.toggleReasoningMode();
                        } else if (ctrl && event.key === Qt.Key_Comma) {
                            event.accepted = true;
                            root.toggleSettingsPanel();
                        } else if (ctrl && event.key === Qt.Key_J) {
                            event.accepted = true;
                            historyController.moveHistorySelection(1);
                        } else if (ctrl && event.key === Qt.Key_K) {
                            event.accepted = true;
                            if (!historyController.moveHistorySelection(-1))
                                root.clearConversation();

                        } else if (ctrl && event.key === Qt.Key_R) {
                            event.accepted = true;
                            historyController.loadHistory();
                        }
                    }

                    Text {
                        anchors.fill: parent
                        visible: prompt.text.length === 0
                        text: root.promptPlaceholder
                        color: root.theme.text_muted
                        font.family: root.themeFontFamily
                        font.pixelSize: prompt.font.pixelSize
                        verticalAlignment: Text.AlignVCenter
                    }

                }

                IconButton {
                    id: recentChatsButton

                    anchors.right: escHint.left
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    size: 28
                    visible: root.hasThread && settingsController.showRecentChats
                    enabled: !root.asking
                    symbol: "history"
                    tooltipText: "Back to recent conversations · Ctrl+N"
                    theme: root.theme
                    onClicked: root.showRecentConversations()
                }

                Rectangle {
                    id: escHint

                    anchors.right: parent.right
                    anchors.rightMargin: 16
                    anchors.verticalCenter: parent.verticalCenter
                    width: escHintLabel.implicitWidth + 12
                    height: 20
                    radius: root.cornerRadius(4)
                    color: root.theme.surface

                    Text {
                        id: escHintLabel

                        anchors.centerIn: parent
                        text: "esc"
                        color: root.theme.text_muted
                        font.family: root.themeFontFamily
                        font.pixelSize: root.fontSize("body_small", 11)
                    }

                }

            }

            Rectangle {
                width: parent.width
                height: 1
                color: root.theme.panel_border
                opacity: 0.55
            }

            Item {
                id: contentArea

                width: parent.width
                height: panel.height - root.chromeHeight

                Column {
                    anchors.fill: parent
                    anchors.leftMargin: 16
                    anchors.rightMargin: 16
                    anchors.topMargin: 12
                    anchors.bottomMargin: 12
                    spacing: root.contentSpacing

                    Rectangle {
                        id: attachmentPanel

                        width: parent.width
                        height: 60
                        visible: root.hasAttachment
                        radius: root.cornerRadius(8)
                        color: root.theme.surface

                        Row {
                            anchors.fill: parent
                            anchors.margins: 8
                            spacing: 10

                            Rectangle {
                                width: 44
                                height: 44
                                radius: root.cornerRadius(6)
                                color: root.theme.panel
                                border.color: root.theme.panel_border
                                border.width: 1
                                clip: true

                                Image {
                                    anchors.fill: parent
                                    anchors.margins: 1
                                    source: root.screenshotSource
                                    visible: root.screenshotPath.length > 0
                                    fillMode: Image.PreserveAspectCrop
                                    asynchronous: true
                                }

                                IconMark {
                                    anchors.centerIn: parent
                                    visible: root.screenshotPath.length === 0
                                    symbol: "plus"
                                    color: root.theme.text_muted
                                }

                            }

                            Column {
                                width: parent.width - 44 - removeAttachmentButton.width - parent.spacing * 2
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 2

                                Text {
                                    text: root.screenshotPath.length > 0 ? "Screenshot attached" : "No attachment"
                                    color: root.theme.text
                                    font.family: root.themeFontFamily
                                    font.pixelSize: root.fontSize("subtitle", 13)
                                    elide: Text.ElideRight
                                    width: parent.width
                                }

                                Text {
                                    id: attachmentStatus

                                    color: root.theme.text_muted
                                    font.family: root.themeFontFamily
                                    font.pixelSize: root.fontSize("body_small", 11)
                                    elide: Text.ElideMiddle
                                    width: parent.width
                                    text: root.screenshotPath
                                }

                            }

                            IconButton {
                                id: removeAttachmentButton

                                anchors.verticalCenter: parent.verticalCenter
                                size: 24
                                symbol: "close"
                                tooltipText: "Remove attachment"
                                danger: true
                                theme: root.theme
                                enabled: root.screenshotPath.length > 0
                                onClicked: root.clearAttachment()
                            }

                        }

                    }

                    ScrollView {
                        id: resultScroll

                        width: parent.width
                        visible: root.showResult
                        height: Math.max(120, contentArea.height - root.contentVPad - (attachmentPanel.visible ? attachmentPanel.height + root.contentSpacing : 0) - (historyPanel.visible ? historyPanel.height + root.contentSpacing : 0) - (secretPanel.visible ? secretPanel.height + root.contentSpacing : 0))
                        clip: true
                        ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

                        Column {
                            id: resultColumn

                            width: resultScroll.availableWidth
                            spacing: 18

                            Repeater {
                                // Count-only model preserves delegates while streaming.
                                model: root.displayMessages.length

                                MessageBlock {
                                    readonly property var message: root.displayMessages[index] ?? ({
                                    })

                                    width: resultColumn.width
                                    role: message.role ?? "assistant"
                                    content: message.content ?? ""
                                    pending: message.pending ?? false
                                    theme: root.theme
                                    onCopyRequested: (text) => {
                                        return root.copyMessageText(text);
                                    }
                                }

                            }

                        }

                        ScrollBar.vertical: ScrollBar {
                            id: resultVerticalBar

                            policy: ScrollBar.AsNeeded
                        }

                        Connections {
                            target: resultScroll.contentItem

                            function onContentHeightChanged() {
                                root.keepResultAtBottom();
                            }

                            function onHeightChanged() {
                                root.keepResultAtBottom();
                            }

                            function onContentYChanged() {
                                if (!root.resultPinning)
                                    root.resultFollowBottom = root.resultIsAtBottom();

                            }
                        }

                    }

                    IdleHint {
                        width: parent.width
                        height: Math.max(140, contentArea.height - root.contentVPad - (attachmentPanel.visible ? attachmentPanel.height + root.contentSpacing : 0))
                        visible: root.showIdleHint
                        theme: root.theme
                        apiKeyConfigured: root.hasConfiguredProvider
                        onSuggestionPicked: function(text) {
                            root.usePromptSuggestion(text);
                        }
                        onScreenshotRequested: {
                            root.usePromptSuggestion("What is on this screenshot?");
                            root.captureScreenshotRegion();
                        }
                        onApiKeySetupRequested: root.promptForApiKeySetup()
                    }

                    SettingsPanel {
                        id: secretPanel

                        width: parent.width
                        visible: settingsController.secretsExpanded
                        theme: root.theme
                        fontFamily: root.themeFontFamily
                        showRecentChats: settingsController.showRecentChats
                        saveHistory: settingsController.saveHistory
                        historyRetentionDays: settingsController.historyRetentionDays
                        retentionModel: retentionOptionsModel
                        confirmClearHistory: historyController.confirmClearHistory
                        recentChatsBusy: settingsController.recentChatsBusy
                        saveHistoryBusy: settingsController.saveHistoryBusy
                        retentionBusy: settingsController.retentionBusy
                        clearHistoryBusy: historyController.historyClearProcess.running
                        globalShortcut: settingsController.globalShortcut
                        globalShortcutConfigured: settingsController.globalShortcutConfigured
                        screenshotShortcut: settingsController.screenshotShortcut
                        screenshotShortcutConfigured: settingsController.screenshotShortcutConfigured
                        shortcutRecording: settingsController.shortcutRecording
                        shortcutRecordingAction: settingsController.shortcutRecordingAction
                        shortcutBusy: settingsController.shortcutBusy
                        secretConfigured: settingsController.secretConfigured
                        secretSource: settingsController.secretSource
                        secretKeyInput: settingsController.secretKeyInput
                        secretSaveBusy: settingsController.secretSaveBusy
                        secretDeleteBusy: settingsController.secretDeleteBusy
                        openRouterSecretConfigured: settingsController.openRouterSecretConfigured
                        openRouterSecretSource: settingsController.openRouterSecretSource
                        openRouterSecretKeyInput: settingsController.openRouterSecretKeyInput
                        openRouterSecretSaveBusy: settingsController.openRouterSecretSaveBusy
                        openRouterSecretDeleteBusy: settingsController.openRouterSecretDeleteBusy
                        onShowRecentChatsToggled: checked => settingsController.saveShowRecentChats(checked)
                        onSaveHistoryToggled: checked => settingsController.saveSaveHistory(checked)
                        onRetentionSelected: days => settingsController.saveHistoryRetention(days)
                        onClearHistoryRequested: historyController.clearAllHistory()
                        onShortcutRecordRequested: action => settingsController.beginShortcutRecording(action)
                        onShortcutDisableRequested: action => settingsController.removeGlobalShortcut(action)
                        onShortcutKeyPressed: event => settingsController.captureGlobalShortcut(event)
                        onSecretInputChanged: text => settingsController.secretKeyInput = text
                        onSecretSaveRequested: settingsController.saveOpenAISecret()
                        onSecretDeleteRequested: settingsController.deleteOpenAISecret()
                        onOpenRouterSecretInputChanged: text => settingsController.openRouterSecretKeyInput = text
                        onOpenRouterSecretSaveRequested: settingsController.saveOpenRouterSecret()
                        onOpenRouterSecretDeleteRequested: settingsController.deleteOpenRouterSecret()
                        onCancelRequested: root.cancelOrClose()
                    }

                    Item {
                        id: historyPanel

                        width: parent.width
                        height: root.historyPanelTargetHeight
                        visible: root.showHistorySuggestions

                        Column {
                            anchors.fill: parent
                            spacing: 4

                            Item {
                                width: parent.width
                                height: 20

                                Text {
                                    anchors.left: parent.left
                                    anchors.leftMargin: 10
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: "RECENT"
                                    color: root.theme.text_muted
                                    font.family: root.themeFontFamily
                                    font.pixelSize: root.fontSize("caption", 10)
                                    font.letterSpacing: 1.2
                                    font.weight: Font.DemiBold
                                }

                            }

                            ListView {
                                width: parent.width
                                height: parent.height - 24
                                clip: true
                                model: historyController.historyModel
                                spacing: 2
                                currentIndex: historyController.historySelectionIndex
                                onCurrentIndexChanged: {
                                    if (currentIndex >= 0)
                                        positionViewAtIndex(currentIndex, ListView.Contain);

                                }

                                delegate: HistoryRow {
                                    width: ListView.view.width
                                    selected: index === historyController.historySelectionIndex
                                    promptText: String(prompt ?? "")
                                    responseText: String(response ?? "")
                                    historyId: Number(entryId ?? 0)
                                    deleting: historyController.historyDeleteProcess.running
                                    theme: root.theme
                                    onEntered: historyController.historySelectionIndex = index
                                    onOpenRequested: function(entryId) {
                                        historyController.openHistoryEntry(entryId);
                                    }
                                    onDeleteRequested: function(entryId) {
                                        historyController.deleteHistoryEntry(entryId);
                                    }
                                }

                            }

                        }

                    }

                }

            }

            Rectangle {
                width: parent.width
                height: 1
                visible: root.hasConfiguredProvider && !root.contentCollapsed
                color: root.theme.panel_border
                opacity: 0.55
            }

            Item {
                width: parent.width
                height: root.hasConfiguredProvider ? 44 : 0
                visible: root.hasConfiguredProvider

                StatusPill {
                    anchors.left: parent.left
                    anchors.leftMargin: 18
                    anchors.right: footerControls.left
                    anchors.rightMargin: 12
                    anchors.verticalCenter: parent.verticalCenter
                    height: 28
                    statusText: root.statusText
                    danger: root.errorText.length > 0
                    theme: root.theme
                }

                Row {
                    id: footerControls

                    anchors.right: parent.right
                    anchors.rightMargin: 10
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 4

                    IconButton {
                        anchors.verticalCenter: parent.verticalCenter
                        symbol: "paperclip"
                        tooltipText: "Attach screenshot region · Ctrl+Shift+C"
                        theme: root.theme
                        visible: root.showAttachAction
                        enabled: !screenshotProcess.running
                        onClicked: root.captureScreenshotRegion()
                    }

                    IconButton {
                        anchors.verticalCenter: parent.verticalCenter
                        symbol: "sliders"
                        tooltipText: (settingsController.secretsExpanded ? "Hide settings" : "Open settings") + " · Ctrl+,"
                        theme: root.theme
                        primary: !settingsController.secretConfigured && !settingsController.openRouterSecretConfigured
                        enabled: !settingsController.secretStatusBusy
                        onClicked: root.toggleSettingsPanel()
                    }

                    Rectangle {
                        width: 1
                        height: 16
                        anchors.verticalCenter: parent.verticalCenter
                        color: root.theme.panel_border
                    }

                    ModelPicker {
                        id: reasoningSelector

                        width: 96
                        anchors.verticalCenter: parent.verticalCenter
                        model: settingsController.reasoningModesModel
                        valueField: "effort"
                        tooltipText: "Choose reasoning effort · Ctrl+T"
                        theme: root.theme
                        enabled: settingsController.reasoningModesModel.count > 0 && !root.asking
                        Component.onCompleted: {
                            root.reasoningSelectorReady = true;
                            root.syncReasoningCombo();
                        }
                        onActivated: {
                            settingsController.selectedReasoningEffort = String(currentValue ?? "");
                            settingsController.saveSelectedReasoningEffort();
                        }
                    }

                    ModelPicker {
                        id: modelSelector

                        width: 128
                        anchors.verticalCenter: parent.verticalCenter
                        model: settingsController.modelsModel
                        tooltipText: "Choose model · Ctrl+M"
                        theme: root.theme
                        enabled: settingsController.modelsModel.count > 0 && !root.asking
                        Component.onCompleted: {
                            root.modelSelectorReady = true;
                            root.syncModelCombo();
                        }
                        onActivated: {
                            settingsController.selectedModel = String(currentValue ?? "");
                            settingsController.saveSelectedModel();
                            settingsController.loadReasoningModes();
                        }
                    }

                    Rectangle {
                        width: 1
                        height: 16
                        anchors.verticalCenter: parent.verticalCenter
                        color: root.theme.panel_border
                    }

                    IconButton {
                        anchors.verticalCenter: parent.verticalCenter
                        symbol: "copy"
                        tooltipText: "Copy latest answer"
                        theme: root.theme
                        visible: root.showCopyAction
                        enabled: root.answerText.length > 0 && !copyTextProcess.running
                        onClicked: root.copyLatest()
                    }

                    PaletteButton {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "Retry"
                        symbol: "refresh"
                        tooltipText: "Retry failed request"
                        theme: root.theme
                        visible: root.showRetryAction
                        primary: true
                        enabled: root.showRetryAction
                        onClicked: root.retryLastPrompt()
                    }

                    PaletteButton {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "Paste"
                        hint: "⌃↵"
                        tooltipText: "Paste latest answer into previous window · Ctrl+Enter"
                        theme: root.theme
                        primary: root.hasAnswer && prompt.text.trim().length === 0 && !root.asking
                        filled: true
                        visible: root.showPasteAction
                        enabled: root.answerText.length > 0 && !pasteProcess.running
                        onClicked: root.pasteLatest()
                    }

                    PaletteButton {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.asking ? "Stop" : "Ask"
                        hint: root.asking ? "esc" : "↵"
                        tooltipText: root.asking ? "Stop streaming · Esc" : "Send prompt · Enter"
                        theme: root.theme
                        primary: root.canSendPrompt
                        danger: root.asking
                        filled: true
                        visible: root.showAskAction
                        enabled: root.asking || root.canSendPrompt
                        onClicked: {
                            if (root.asking)
                                root.stopRequest();
                            else
                                root.sendPrompt();
                        }
                    }

                }

            }

        }

        Behavior on height {
            NumberAnimation {
                duration: 140
                easing.type: Easing.OutCubic
            }

        }

        Behavior on opacity {
            NumberAnimation {
                duration: 120
                easing.type: Easing.OutCubic
            }

        }

        Behavior on scale {
            NumberAnimation {
                duration: 120
                easing.type: Easing.OutCubic
            }

        }

    }

}
