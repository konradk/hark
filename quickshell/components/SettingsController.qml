import QtQuick
import Quickshell.Io

QtObject {
    id: root

    required property var app
    property string harkctlPath: "harkctl"
    property string shortcutIntegration: "hyprland"

    property string selectedModel: ""
    property string selectedReasoningEffort: "low"
    property bool reasoningSettingLoaded: false
    property var modelCatalog: []
    readonly property string selectedProvider: providerForModel(selectedModel)
    property var modelsModel: ListModel {
    }

    property var reasoningModesModel: ListModel {
    }

    property bool showRecentChats: true
    property bool saveHistory: true
    property int historyRetentionDays: 0
    property bool secretsExpanded: false
    property alias secretConfigured: openAISecretItem.configured
    property alias secretSource: openAISecretItem.source
    property alias secretKeyInput: openAISecretItem.keyInput
    property alias openRouterSecretConfigured: openRouterSecretItem.configured
    property alias openRouterSecretSource: openRouterSecretItem.source
    property alias openRouterSecretKeyInput: openRouterSecretItem.keyInput
    property alias xAISecretConfigured: xAISecretItem.configured
    property alias xAISecretSource: xAISecretItem.source
    property alias xAISecretKeyInput: xAISecretItem.keyInput
    property bool globalShortcutConfigured: false
    property string globalShortcut: ""
    property bool screenshotShortcutConfigured: false
    property string screenshotShortcut: ""
    property string pendingGlobalShortcut: ""
    property string pendingShortcutAction: ""
    property bool shortcutRecording: false
    property string shortcutRecordingAction: ""
    property string shortcutProcessError: ""

    readonly property bool recentChatsBusy: recentChatsSetting.busy
    readonly property bool saveHistoryBusy: saveHistorySetting.busy
    readonly property bool retentionBusy: retentionSetting.busy
    readonly property bool shortcutBusy: shortcutSetProcess.running
    readonly property bool secretStatusBusy: openAISecret.statusBusy || openRouterSecret.statusBusy || xAISecret.statusBusy
    readonly property bool secretSaveBusy: openAISecret.saveBusy
    readonly property bool secretDeleteBusy: openAISecret.deleteBusy
    readonly property bool openRouterSecretSaveBusy: openRouterSecret.saveBusy
    readonly property bool openRouterSecretDeleteBusy: openRouterSecret.deleteBusy
    readonly property bool xAISecretSaveBusy: xAISecret.saveBusy
    readonly property bool xAISecretDeleteBusy: xAISecret.deleteBusy

    function loadModels() {
        if (!modelsProcess.running)
            modelsProcess.exec([harkctlPath, "models", "--json"]);

    }

    function handleModels(line) {
        if (line.length === 0)
            return ;

        try {
            const models = JSON.parse(line);
            const catalog = [];
            for (const model of models) {
                const modelId = model.id ?? "";
                if (modelId.length === 0)
                    continue;

                catalog.push({
                    "modelId": modelId,
                    "label": model.label ?? modelId,
                    "provider": String(model.provider ?? ""),
                    "reasoningEfforts": Array.isArray(model.reasoning_efforts) ? model.reasoning_efforts.map(effort => String(effort)) : ["auto"]
                });
            }
            modelCatalog = catalog;
            refreshAvailableModels();
            loadReasoningModes();
        } catch (error) {
            app.statusText = "Model list parse failed";
        }
    }

    function providerConfigured(provider) {
        if (provider === "openai")
            return secretConfigured;

        if (provider === "openrouter")
            return openRouterSecretConfigured;

        if (provider === "xai")
            return xAISecretConfigured;

        // An unrecognized provider comes from the user's own config and is
        // resolved by the daemon, so never hide it here.
        return true;
    }

    function providerForModel(modelId) {
        for (const model of modelCatalog) {
            if (model.modelId === modelId)
                return model.provider;
        }
        return "";
    }

    function indexOfAvailableModel(modelId) {
        for (let index = 0; index < modelsModel.count; index++) {
            if (modelsModel.get(index).modelId === modelId)
                return index;

        }
        return -1;
    }

    // A model whose provider has no key fails only after the prompt was sent,
    // so keep those out of the picker and off a stale stored selection.
    // Returns whether the effective selection had to change.
    function refreshAvailableModels() {
        modelsModel.clear();
        for (const model of modelCatalog) {
            if (!providerConfigured(model.provider))
                continue;

            modelsModel.append({
                "modelId": model.modelId,
                "label": model.label
            });
        }
        const selectionStale = modelsModel.count > 0 && indexOfAvailableModel(selectedModel) < 0;
        if (selectionStale)
            // Deliberately not saved: the stored choice must come back when its
            // provider key does.
            selectedModel = modelsModel.get(0).modelId;

        app.syncModelCombo();
        return selectionStale;
    }

    function loadReasoningModes() {
        reasoningModesModel.clear();
        for (const model of modelCatalog) {
            if (model.modelId !== selectedModel)
                continue;

            for (const effort of model.reasoningEfforts) {
                reasoningModesModel.append({
                    "label": effort === "xhigh" ? "XHigh" : effort.charAt(0).toUpperCase() + effort.slice(1),
                    "effort": effort
                });
            }
            break;
        }
        ensureSupportedReasoningEffort();
    }

    function ensureSupportedReasoningEffort() {
        if (reasoningModesModel.count === 0)
            return ;

        for (let index = 0; index < reasoningModesModel.count; index++) {
            if (reasoningModesModel.get(index).effort === selectedReasoningEffort) {
                app.syncReasoningCombo();
                return ;
            }
        }

        let fallback = reasoningModesModel.get(0).effort;
        for (let index = 0; index < reasoningModesModel.count; index++) {
            if (reasoningModesModel.get(index).effort === "auto") {
                fallback = "auto";
                break;
            }
        }
        selectedReasoningEffort = fallback;
        app.syncReasoningCombo();
        if (reasoningSettingLoaded)
            saveSelectedReasoningEffort();
    }

    function loadSelectedModel() {
        modelSetting.load();
    }

    function saveSelectedModel() {
        if (selectedModel.length > 0)
            modelSetting.save(selectedModel);

    }

    function loadSelectedReasoningEffort() {
        reasoningSetting.load();
    }

    function saveSelectedReasoningEffort() {
        if (selectedReasoningEffort.length > 0)
            reasoningSetting.save(selectedReasoningEffort);

    }

    function applyShowRecentChats(enabled) {
        showRecentChats = enabled;
        if (!enabled)
            app.history.reset();
        else
            app.history.loadHistory();
    }

    function loadShowRecentChats() {
        recentChatsSetting.load();
    }

    function saveShowRecentChats(enabled) {
        if (recentChatsSetting.busy)
            return ;

        applyShowRecentChats(enabled);
        recentChatsSetting.save(enabled ? "true" : "false");
    }

    function loadSaveHistory() {
        saveHistorySetting.load();
    }

    function saveSaveHistory(enabled) {
        if (saveHistorySetting.busy)
            return ;

        saveHistory = enabled;
        saveHistorySetting.save(enabled ? "true" : "false");
    }

    function loadHistoryRetention() {
        retentionSetting.load();
    }

    function saveHistoryRetention(days) {
        const value = Number(days);
        if (retentionSetting.busy || !isFinite(value) || value < 0)
            return ;

        historyRetentionDays = Math.round(value);
        retentionSetting.save(historyRetentionDays);
    }

    function loadSecretStatus() {
        openAISecret.loadStatus();
        openRouterSecret.loadStatus();
        xAISecret.loadStatus();
    }

    function saveOpenAISecret() {
        openAISecret.save();
    }

    function deleteOpenAISecret() {
        openAISecret.remove();
    }

    function saveOpenRouterSecret() {
        openRouterSecret.save();
    }

    function deleteOpenRouterSecret() {
        openRouterSecret.remove();
    }

    function saveXAISecret() {
        xAISecret.save();
    }

    function deleteXAISecret() {
        xAISecret.remove();
    }

    function loadGlobalShortcut() {
        if (!shortcutGetProcess.running)
            shortcutGetProcess.exec([harkctlPath, "shortcut", "get", "--json", "--integration", shortcutIntegration]);

        if (!screenshotShortcutGetProcess.running)
            screenshotShortcutGetProcess.exec([harkctlPath, "shortcut", "get", "--json", "--integration", shortcutIntegration, "--action", "screenshot"]);

    }

    function handleGlobalShortcut(line) {
        if (line.length === 0)
            return ;

        try {
            const status = JSON.parse(line);
            if (String(status.action ?? "open") === "screenshot") {
                screenshotShortcutConfigured = Boolean(status.configured);
                screenshotShortcut = screenshotShortcutConfigured ? String(status.shortcut ?? "") : "";
            } else {
                globalShortcutConfigured = Boolean(status.configured);
                globalShortcut = globalShortcutConfigured ? String(status.shortcut ?? "") : "";
            }
        } catch (error) {
            app.statusText = "Shortcut status parse failed";
        }
    }

    function beginShortcutRecording(action) {
        if (shortcutSetProcess.running)
            return ;

        shortcutRecording = true;
        shortcutRecordingAction = action;
        shortcutProcessError = "";
        app.statusText = "Press a modifier plus another key";
        app.beginSecretPanelShortcutRecording(action === "screenshot" ? "screenshot" : "open");
    }

    function cancelShortcutRecording() {
        shortcutRecording = false;
        shortcutRecordingAction = "";
        app.statusText = "";
        app.focusSecretPanelInput();
    }

    function shortcutKeyName(key) {
        if (key >= Qt.Key_A && key <= Qt.Key_Z)
            return String.fromCharCode(key);

        if (key >= Qt.Key_0 && key <= Qt.Key_9)
            return String.fromCharCode(key);

        if (key >= Qt.Key_F1 && key <= Qt.Key_F24)
            return "F" + String(key - Qt.Key_F1 + 1);

        switch (key) {
        case Qt.Key_Space:
            return "SPACE";
        case Qt.Key_Return:
        case Qt.Key_Enter:
            return "RETURN";
        case Qt.Key_Tab:
            return "TAB";
        case Qt.Key_Backspace:
            return "BACKSPACE";
        case Qt.Key_Delete:
            return "DELETE";
        case Qt.Key_Insert:
            return "INSERT";
        case Qt.Key_Home:
            return "HOME";
        case Qt.Key_End:
            return "END";
        case Qt.Key_PageUp:
            return "PAGEUP";
        case Qt.Key_PageDown:
            return "PAGEDOWN";
        case Qt.Key_Left:
            return "LEFT";
        case Qt.Key_Right:
            return "RIGHT";
        case Qt.Key_Up:
            return "UP";
        case Qt.Key_Down:
            return "DOWN";
        case Qt.Key_Minus:
            return "MINUS";
        case Qt.Key_Equal:
            return "EQUAL";
        case Qt.Key_Comma:
            return "COMMA";
        case Qt.Key_Period:
            return "PERIOD";
        case Qt.Key_Slash:
            return "SLASH";
        case Qt.Key_Semicolon:
            return "SEMICOLON";
        case Qt.Key_Apostrophe:
            return "APOSTROPHE";
        case Qt.Key_BracketLeft:
            return "BRACKETLEFT";
        case Qt.Key_BracketRight:
            return "BRACKETRIGHT";
        case Qt.Key_Backslash:
            return "BACKSLASH";
        case Qt.Key_QuoteLeft:
            return "GRAVE";
        default:
            return "";
        }
    }

    function captureGlobalShortcut(event) {
        if (!shortcutRecording)
            return ;

        if (event.key === Qt.Key_Escape) {
            cancelShortcutRecording();
            return ;
        }

        if (event.key === Qt.Key_Meta || event.key === Qt.Key_Super_L || event.key === Qt.Key_Super_R || event.key === Qt.Key_Control || event.key === Qt.Key_Shift || event.key === Qt.Key_Alt)
            return ;

        const hasSuper = event.modifiers & Qt.MetaModifier;
        const hasControl = event.modifiers & Qt.ControlModifier;
        const hasAlt = event.modifiers & Qt.AltModifier;
        if (!hasSuper && !hasControl && !hasAlt) {
            app.statusText = "Use Super, Ctrl, or Alt with another key";
            return ;
        }

        const keyName = shortcutKeyName(event.key);
        if (keyName.length === 0) {
            app.statusText = "That key is not supported";
            return ;
        }

        const parts = [];
        if (hasSuper)
            parts.push("SUPER");

        if (hasControl)
            parts.push("CTRL");

        if (event.modifiers & Qt.ShiftModifier)
            parts.push("SHIFT");

        if (hasAlt)
            parts.push("ALT");

        parts.push(keyName);
        saveGlobalShortcut(shortcutRecordingAction, parts.join(" + "));
    }

    function saveGlobalShortcut(action, shortcut) {
        if (shortcutSetProcess.running)
            return ;

        shortcutRecording = false;
        shortcutRecordingAction = "";
        pendingGlobalShortcut = shortcut;
        pendingShortcutAction = action;
        shortcutProcessError = "";
        app.statusText = "Saving global shortcut...";
        shortcutSetProcess.exec([harkctlPath, "shortcut", "set", "--json", "--integration", shortcutIntegration, "--action", action, "--shell", app.shellConfigPath, shortcut]);
    }

    function removeGlobalShortcut(action) {
        const configured = action === "screenshot" ? screenshotShortcutConfigured : globalShortcutConfigured;
        if (shortcutSetProcess.running || !configured)
            return ;

        shortcutRecording = false;
        shortcutRecordingAction = "";
        pendingGlobalShortcut = "";
        pendingShortcutAction = action;
        shortcutProcessError = "";
        app.statusText = "Disabling global shortcut...";
        shortcutSetProcess.exec([harkctlPath, "shortcut", "remove", "--json", "--integration", shortcutIntegration, "--action", action]);
    }

    function loadDoctor() {
        if (!doctorProcess.running)
            doctorProcess.exec([harkctlPath, "doctor", "--json"]);

    }

    function handleDoctor(line) {
        if (line.length === 0)
            return ;

        try {
            const statuses = JSON.parse(line);
            const missing = [];
            for (const status of statuses) {
                if (!status.found)
                    missing.push(status.name);

            }
            if (missing.length > 0)
                app.statusText = "Missing: " + missing.join(", ");

        } catch (error) {
            app.statusText = "Dependency check failed";
        }
    }

    property SettingProcess modelSetting: SettingProcess {
        id: modelSettingItem

        app: root.app
        harkctlPath: root.harkctlPath
        key: "selected_model"
        failureText: "Could not save model"
        onLoaded: (value, found) => {
            if (found && value) {
                root.selectedModel = String(value);
                // The stored model may belong to a provider whose key is gone.
                root.refreshAvailableModels();
                root.loadReasoningModes();
            }
        }
    }

    property SettingProcess reasoningSetting: SettingProcess {
        id: reasoningSettingItem

        app: root.app
        harkctlPath: root.harkctlPath
        key: "selected_reasoning_effort"
        failureText: "Could not save reasoning effort"
        onLoaded: (value, found) => {
            root.reasoningSettingLoaded = true;
            if (found && value) {
                root.selectedReasoningEffort = String(value);
            }
            root.ensureSupportedReasoningEffort();
        }
    }

    property SettingProcess recentChatsSetting: SettingProcess {
        id: recentChatsSettingItem

        app: root.app
        harkctlPath: root.harkctlPath
        key: "show_recent_chats"
        savingText: "Saving setting..."
        onLoaded: (value, found) => {
            root.applyShowRecentChats(!found || String(value).toLowerCase() !== "false");
        }
        onSaved: recentChatsSettingItem.reportSaved("")
    }

    property SettingProcess saveHistorySetting: SettingProcess {
        id: saveHistorySettingItem

        app: root.app
        harkctlPath: root.harkctlPath
        key: "save_history"
        savingText: "Saving history setting..."
        failureText: "Could not save history setting"
        onLoaded: (value, found) => {
            root.saveHistory = !found || String(value).toLowerCase() !== "false";
        }
        onSaved: saveHistorySettingItem.reportSaved(root.saveHistory ? "History saving enabled" : "History saving disabled")
    }

    property SettingProcess retentionSetting: SettingProcess {
        id: retentionSettingItem

        app: root.app
        harkctlPath: root.harkctlPath
        key: "history_retention_days"
        savingText: "Saving retention..."
        failureText: "Could not save retention"
        onLoaded: (value, found) => {
            const days = Number(value ?? 0);
            root.historyRetentionDays = isFinite(days) && days >= 0 ? Math.round(days) : 0;
            root.app.syncRetentionCombo();
        }
        onSaved: retentionSettingItem.reportSaved("History retention updated")
        // A retention change prunes on the daemon side, so the recent list must
        // refresh whether or not the save itself succeeded.
        Component.onCompleted: retentionSettingItem.setProcess.exited.connect(() => root.app.history.loadHistory())
    }

    property SecretProcess openAISecret: SecretProcess {
        id: openAISecretItem

        app: root.app
        harkctlPath: root.harkctlPath
        provider: "openai"
        label: "OpenAI"
        // Adding or deleting a key changes which models can answer at all.
        onConfiguredChanged: {
            if (root.refreshAvailableModels())
                root.loadReasoningModes();

        }
    }

    property SecretProcess openRouterSecret: SecretProcess {
        id: openRouterSecretItem

        app: root.app
        harkctlPath: root.harkctlPath
        provider: "openrouter"
        label: "OpenRouter"
        onConfiguredChanged: {
            if (root.refreshAvailableModels())
                root.loadReasoningModes();

        }
    }

    property SecretProcess xAISecret: SecretProcess {
        id: xAISecretItem

        app: root.app
        harkctlPath: root.harkctlPath
        provider: "xai"
        label: "xAI"
        onConfiguredChanged: {
            if (root.refreshAvailableModels())
                root.loadReasoningModes();

        }
    }

    property Process modelsProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                return root.handleModels(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    console.warn("Hark model list refresh failed:", line.replace(/^harkctl:\s*/, ""));

            }
        }

    }

    property Process shortcutGetProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                return root.handleGlobalShortcut(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

    property Process screenshotShortcutGetProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                return root.handleGlobalShortcut(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

    property Process shortcutSetProcess: Process {
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0) {
                const actionLabel = root.pendingShortcutAction === "screenshot" ? "Screenshot shortcut" : "Open shortcut";
                const currentShortcut = root.pendingShortcutAction === "screenshot" ? root.screenshotShortcut : root.globalShortcut;
                root.app.statusText = root.pendingGlobalShortcut.length > 0 ? actionLabel + " set to " + currentShortcut : actionLabel + " disabled";
            } else if (root.shortcutProcessError.length === 0) {
                root.app.statusText = "Could not change global shortcut";
            }
            root.pendingGlobalShortcut = "";
            root.pendingShortcutAction = "";
        }

        stdout: SplitParser {
            onRead: (line) => {
                return root.handleGlobalShortcut(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0) {
                    root.shortcutProcessError = line;
                    root.app.statusText = line.replace(/^harkctl:\s*/, "");
                }
            }
        }

    }

    property Process doctorProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                return root.handleDoctor(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

}
