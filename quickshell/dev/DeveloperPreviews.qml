import QtQuick
import "PreviewFixtures.js" as PreviewFixtures

// Developer-only preview states. Loaded only when HARK_ENABLE_PREVIEWS=1, so
// none of this reaches a normal session. See docs/visual-verification.md.
QtObject {
    id: previews

    property var app: null

    function show(name) {
        if (!app)
            return;

        switch (name) {
        case "idle":
            idleState();
            break;
        case "reset":
            resetState();
            break;
        case "thread":
            threadState();
            break;
        case "code":
            codeState();
            break;
        case "markdown":
            markdownState();
            break;
        case "typing":
            typingState();
            break;
        case "streaming":
            streamingState();
            break;
        case "settings":
            settingsState();
            break;
        case "history":
            historyState();
            break;
        case "demo-history":
            demoHistoryState();
            break;
        case "demo-conversation":
            demoConversationState();
            break;
        case "demo-attachment":
            demoAttachmentState();
            break;
        case "theme-tokyo-night":
            promptThemeState("tokyo-night");
            break;
        case "theme-catppuccin-latte":
            promptThemeState("catppuccin-latte");
            break;
        case "theme-solitude":
            promptThemeState("solitude");
            break;
        case "theme-nord":
            promptThemeState("nord");
            break;
        }
    }

    function applyDemoTheme(name) {
        const fixture = PreviewFixtures.theme(name);
        app.theme = Object.assign({}, app.theme, fixture.colors);
        app.previewBackdropColor = fixture.backdrop;
        app.preferSystemTheme = false;
    }

    function prepareDemo(name) {
        app.resetForPreview();
        app.history.historyModel.clear();
        app.settings.selectedModel = "gpt-5.6-sol";
        app.settings.selectedReasoningEffort = "medium";
        app.previewProviderConfiguredOverride = true;
        applyDemoTheme(name);
    }

    function idleState() {
        app.resetForPreview();
        app.history.historyModel.clear();
        app.previewProviderConfiguredOverride = false;
        applyDemoTheme("tokyo-night");
        app.revealPreview();
    }

    function resetState() {
        app.resetForPreview();
        app.history.historyModel.clear();
        app.history.loadHistory();
        app.hideOverlay(false);
    }

    function applyConversation(fixture) {
        app.answerText = fixture.answer;
        app.threadMessages = [{
            "role": "user",
            "content": fixture.prompt
        }, {
            "role": "assistant",
            "content": fixture.answer
        }];
        app.renderThread();
        app.revealPreview();
    }

    function threadState() {
        app.resetForPreview();
        applyConversation(PreviewFixtures.longThread());
    }

    function codeState() {
        app.resetForPreview();
        applyConversation(PreviewFixtures.code());
    }

    function markdownState() {
        app.resetForPreview();
        applyConversation(PreviewFixtures.markdown());
    }

    function typingState() {
        app.resetForPreview();
        app.usePromptSuggestion("how do i resize a window");
        app.revealPreview();
    }

    function streamingState() {
        app.resetForPreview();
        app.previewAsking = true;
        app.threadMessages = [{
            "role": "user",
            "content": "jak dojade we wroclawiu tramwajem z zoo do rynku"
        }];
        app.renderThread("");
        app.revealPreview();
    }

    function settingsState() {
        app.resetForPreview();
        const settings = app.settings;
        settings.selectedModel = settings.selectedModel.length > 0 ? settings.selectedModel : "gpt-5.6-sol";
        settings.selectedReasoningEffort = settings.selectedReasoningEffort.length > 0 ? settings.selectedReasoningEffort : "low";
        settings.secretConfigured = true;
        settings.secretSource = "secret-service";
        settings.secretsExpanded = true;
        settings.loadGlobalShortcut();
        app.revealPreview();
    }

    function historyState() {
        app.resetForPreview();
        const fixture = PreviewFixtures.history();
        app.history.historyModel.clear();
        for (const entry of fixture.entries)
            app.history.historyModel.append(entry);

        app.renderThread();
        app.revealPreview();
    }

    function demoHistoryState() {
        prepareDemo("tokyo-night");
        const fixture = PreviewFixtures.demoHistory();
        for (const entry of fixture.entries)
            app.history.historyModel.append(entry);

        app.renderThread();
        app.revealPreview();
    }

    function demoConversationState() {
        prepareDemo("solitude");
        applyConversation(PreviewFixtures.demoConversation());
    }

    function demoAttachmentState() {
        prepareDemo("nord");
        app.screenshotPath = "/tmp/hark/screenshots/window-editor.png";
        app.screenshotSourceOverride = Qt.resolvedUrl("demo-screenshot.svg");
        applyConversation(PreviewFixtures.demoAttachment());
    }

    function promptThemeState(name) {
        prepareDemo(name);
        app.usePromptSuggestion("Summarize the active window and suggest next steps");
        app.revealPreview();
    }
}
