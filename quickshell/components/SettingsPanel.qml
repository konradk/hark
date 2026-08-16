import QtQuick

Rectangle {
    id: panel

    property var theme: null
    property string fontFamily: ""
    property bool showRecentChats: true
    property bool saveHistory: true
    property int historyRetentionDays: 0
    property var retentionModel: null
    property bool confirmClearHistory: false
    property bool recentChatsBusy: false
    property bool saveHistoryBusy: false
    property bool retentionBusy: false
    property bool clearHistoryBusy: false
    property bool barWidgetVisible: true
    property bool barWidgetBusy: false
    property string globalShortcut: ""
    property bool globalShortcutConfigured: false
    property string screenshotShortcut: ""
    property bool screenshotShortcutConfigured: false
    property bool shortcutRecording: false
    property string shortcutRecordingAction: ""
    property bool shortcutBusy: false
    property bool secretConfigured: false
    property string secretSource: "none"
    property string secretKeyInput: ""
    property bool secretSaveBusy: false
    property bool secretDeleteBusy: false
    property bool openRouterSecretConfigured: false
    property string openRouterSecretSource: "none"
    property string openRouterSecretKeyInput: ""
    property bool openRouterSecretSaveBusy: false
    property bool openRouterSecretDeleteBusy: false
    property bool xAISecretConfigured: false
    property string xAISecretSource: "none"
    property string xAISecretKeyInput: ""
    property bool xAISecretSaveBusy: false
    property bool xAISecretDeleteBusy: false
    property var providersModel: null
    property bool providersBusy: false
    property bool providerAddBusy: false
    property bool providerFormVisible: false
    property string editingProviderID: ""
    property string providerFormLabel: ""
    property string providerFormBaseURL: ""
    property string providerFormKey: ""
    property var providerFormModels: []

    readonly property real labelWidth: 180

    signal showRecentChatsToggled(bool checked)
    signal saveHistoryToggled(bool checked)
    signal retentionSelected(int days)
    signal clearHistoryRequested()
    signal barWidgetToggled(bool checked)
    signal shortcutRecordRequested(string action)
    signal shortcutDisableRequested(string action)
    signal shortcutKeyPressed(var event)
    signal secretInputChanged(string text)
    signal secretSaveRequested()
    signal secretDeleteRequested()
    signal openRouterSecretInputChanged(string text)
    signal openRouterSecretSaveRequested()
    signal openRouterSecretDeleteRequested()
    signal xAISecretInputChanged(string text)
    signal xAISecretSaveRequested()
    signal xAISecretDeleteRequested()
    signal providerSaveRequested(string id, string label, string baseURL, string key, var models)
    signal providerRemoveRequested(string id)
    signal cancelRequested()

    function c(name, fallback) {
        return theme && theme[name] ? theme[name] : fallback;
    }

    function cornerRadius(maximum) {
        const configured = theme && theme.corner_radius !== undefined ? Number(theme.corner_radius) : maximum;
        return Math.min(Math.max(0, configured), maximum);
    }

    function fontSize(role, fallback) {
        const configured = theme ? Number(theme["font_" + role]) : NaN;
        return isFinite(configured) && configured > 0 ? configured : fallback;
    }

    function focusSecretInput() {
        openAISecretRow.focusInput();
    }

    function resetProviderForm() {
        providerFormVisible = false;
        editingProviderID = "";
        providerFormLabel = "";
        providerFormBaseURL = "";
        providerFormKey = "";
        providerFormModels = [];
    }

    function beginAddProvider() {
        resetProviderForm();
        providerFormVisible = true;
    }

    function beginEditProvider(id, label, baseURL, models) {
        editingProviderID = String(id ?? "");
        providerFormLabel = String(label ?? "");
        providerFormBaseURL = String(baseURL ?? "");
        providerFormKey = "";
        providerFormModels = Array.isArray(models) ? models.map(model => String(model)) : [];
        providerFormVisible = true;
    }

    function addFormModel(id) {
        const trimmed = String(id).trim();
        if (trimmed.length === 0 || providerFormModels.indexOf(trimmed) >= 0)
            return ;

        providerFormModels = providerFormModels.concat([trimmed]);
    }

    function removeFormModel(id) {
        providerFormModels = providerFormModels.filter(model => model !== id);
    }

    function addFormModelFromInput() {
        if (providerModelInputField.text.trim().length === 0)
            return ;

        addFormModel(providerModelInputField.text);
        providerModelInputField.text = "";
    }

    function beginShortcutRecording(action) {
        if (action === "screenshot")
            screenshotShortcutRow.beginRecording();
        else
            openShortcutRow.beginRecording();
    }

    function syncRetention() {
        if (!retentionSelector.ready || !retentionModel)
            return;

        let index = retentionSelector.indexOfValue(historyRetentionDays);
        if (index < 0 && historyRetentionDays > 0) {
            retentionModel.append({
                "label": historyRetentionDays + " days",
                "days": historyRetentionDays
            });
            index = retentionModel.count - 1;
        }
        retentionSelector.currentIndex = index >= 0 ? index : 0;
    }

    width: parent ? parent.width : 0
    implicitHeight: content.implicitHeight + 28
    height: implicitHeight
    radius: cornerRadius(10)
    color: c("surface", "#11141a")
    onHistoryRetentionDaysChanged: syncRetention()
    onVisibleChanged: {
        if (visible)
            return;

        openAISecretRow.stopEditing();
        openRouterSecretRow.stopEditing();
        xAISecretRow.stopEditing();
        panel.resetProviderForm();
    }

    Column {
        id: content

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: 14
        spacing: 4

        ToggleSettingRow {
            width: parent.width
            title: "Show Hark icon in the bar"
            hint: "Hide the Hark button from the Omarchy bar if you open it with a shortcut"
            checked: panel.barWidgetVisible
            busy: panel.barWidgetBusy
            theme: panel.theme
            onToggled: checked => panel.barWidgetToggled(checked)
        }

        Text {
            width: parent.width
            text: "CHATS"
            color: panel.c("text_muted", "#8a93a3")
            font.family: panel.fontFamily
            font.pixelSize: panel.fontSize("caption", 10)
            font.letterSpacing: 1.2
            font.weight: Font.DemiBold
            topPadding: 10
            bottomPadding: 4
        }

        ToggleSettingRow {
            width: parent.width
            title: "Show recent chats on open"
            checked: panel.showRecentChats
            busy: panel.recentChatsBusy
            theme: panel.theme
            onToggled: checked => panel.showRecentChatsToggled(checked)
        }

        ToggleSettingRow {
            width: parent.width
            title: "Save chat history"
            checked: panel.saveHistory
            busy: panel.saveHistoryBusy
            theme: panel.theme
            onToggled: checked => panel.saveHistoryToggled(checked)
        }

        Item {
            width: parent.width
            height: 34
            opacity: panel.saveHistory ? 1 : 0.55

            Text {
                anchors.left: parent.left
                anchors.right: retentionSelector.left
                anchors.rightMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: "Keep chats for"
                color: panel.c("text", "#d3d8e2")
                font.family: panel.fontFamily
                font.pixelSize: panel.fontSize("subtitle", 13)
                elide: Text.ElideRight
            }

            ModelPicker {
                id: retentionSelector

                property bool ready: false

                width: 108
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                model: panel.retentionModel
                valueField: "days"
                tooltipText: "Choose how long chats are kept"
                theme: panel.theme
                enabled: panel.saveHistory && !panel.retentionBusy
                Component.onCompleted: {
                    ready = true;
                    panel.syncRetention();
                }
                onActivated: panel.retentionSelected(Number(currentValue))
            }
        }

        Item {
            width: parent.width
            height: 34

            Text {
                anchors.left: parent.left
                anchors.right: clearAllHistoryButton.left
                anchors.rightMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: panel.confirmClearHistory ? "Delete every chat and cached screenshot?" : "Delete all local data"
                color: panel.confirmClearHistory ? panel.c("error_text", "#ffb4b4") : panel.c("text", "#d3d8e2")
                font.family: panel.fontFamily
                font.pixelSize: panel.fontSize("subtitle", 13)
                elide: Text.ElideRight
            }

            PaletteButton {
                id: clearAllHistoryButton

                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: panel.confirmClearHistory ? "Confirm" : "Clear all"
                tooltipText: "Permanently delete all chats and cached screenshots"
                danger: true
                filled: panel.confirmClearHistory
                theme: panel.theme
                enabled: !panel.clearHistoryBusy
                onClicked: panel.clearHistoryRequested()
            }
        }

        Text {
            width: parent.width
            text: "SHORTCUTS"
            color: panel.c("text_muted", "#8a93a3")
            font.family: panel.fontFamily
            font.pixelSize: panel.fontSize("caption", 10)
            font.letterSpacing: 1.2
            font.weight: Font.DemiBold
            topPadding: 14
            bottomPadding: 4
        }

        ShortcutSettingRow {
            id: openShortcutRow

            width: parent.width
            title: "Open Hark"
            labelWidth: panel.labelWidth
            shortcut: panel.globalShortcut
            configured: panel.globalShortcutConfigured
            recording: panel.shortcutRecording && panel.shortcutRecordingAction === "open"
            busy: panel.shortcutBusy
            theme: panel.theme
            onRecordRequested: panel.shortcutRecordRequested("open")
            onDisableRequested: panel.shortcutDisableRequested("open")
            onShortcutKeyPressed: event => panel.shortcutKeyPressed(event)
        }

        ShortcutSettingRow {
            id: screenshotShortcutRow

            width: parent.width
            title: "Capture active window"
            labelWidth: panel.labelWidth
            shortcut: panel.screenshotShortcut
            configured: panel.screenshotShortcutConfigured
            recording: panel.shortcutRecording && panel.shortcutRecordingAction === "screenshot"
            busy: panel.shortcutBusy
            theme: panel.theme
            onRecordRequested: panel.shortcutRecordRequested("screenshot")
            onDisableRequested: panel.shortcutDisableRequested("screenshot")
            onShortcutKeyPressed: event => panel.shortcutKeyPressed(event)
        }

        Text {
            width: parent.width
            text: "PROVIDERS"
            color: panel.c("text_muted", "#8a93a3")
            font.family: panel.fontFamily
            font.pixelSize: panel.fontSize("caption", 10)
            font.letterSpacing: 1.2
            font.weight: Font.DemiBold
            topPadding: 14
            bottomPadding: 4
        }

        Repeater {
            model: panel.providersModel

            delegate: ProviderRow {
                width: parent.width
                providerId: String(model.id ?? "")
                label: String(model.label ?? model.id ?? "")
                baseUrl: String(model.baseUrl ?? "")
                models: JSON.parse(String(model.modelsJson ?? "[]"))
                busy: panel.providersBusy
                theme: panel.theme
                fontFamily: panel.fontFamily
                onEditRequested: panel.beginEditProvider(model.id, model.label, model.baseUrl, JSON.parse(String(model.modelsJson ?? "[]")))
                onRemoveRequested: panel.providerRemoveRequested(String(model.id ?? ""))
            }
        }

        Item {
            width: parent.width
            height: 34

            PaletteButton {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: panel.providerFormVisible ? "Cancel" : "Add provider"
                tooltipText: "Add an OpenAI-compatible provider"
                theme: panel.theme
                filled: panel.providerFormVisible
                enabled: !panel.providerAddBusy
                onClicked: {
                    if (panel.providerFormVisible)
                        panel.resetProviderForm();
                    else
                        panel.beginAddProvider();
                }
            }
        }

        Column {
            width: parent.width
            visible: panel.providerFormVisible
            spacing: 4

            Item {
                width: parent.width
                height: 32

                Text {
                    width: panel.labelWidth
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Name"
                    color: panel.c("text", "#d3d8e2")
                    font.family: panel.fontFamily
                    font.pixelSize: panel.fontSize("subtitle", 13)
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.leftMargin: panel.labelWidth + 8
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: 30
                    radius: panel.cornerRadius(7)
                    color: panel.c("input", "#0d1016")
                    border.width: 1
                    border.color: providerNameField.activeFocus ? panel.c("primary", "#a7c7ff") : panel.c("panel_border", "#2b303b")

                    TextInput {
                        id: providerNameField

                        anchors.fill: parent
                        anchors.leftMargin: 10
                        anchors.rightMargin: 10
                        text: panel.providerFormLabel
                        color: panel.c("text_strong", "#f2f4f8")
                        font.family: panel.fontFamily
                        font.pixelSize: panel.fontSize("subtitle", 13)
                        verticalAlignment: TextInput.AlignVCenter
                        clip: true
                        onTextChanged: panel.providerFormLabel = text
                    }
                }
            }

            Item {
                width: parent.width
                height: 32

                Text {
                    width: panel.labelWidth
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Base URL"
                    color: panel.c("text", "#d3d8e2")
                    font.family: panel.fontFamily
                    font.pixelSize: panel.fontSize("subtitle", 13)
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.leftMargin: panel.labelWidth + 8
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: 30
                    radius: panel.cornerRadius(7)
                    color: panel.c("input", "#0d1016")
                    border.width: 1
                    border.color: providerBaseUrlField.activeFocus ? panel.c("primary", "#a7c7ff") : panel.c("panel_border", "#2b303b")

                    TextInput {
                        id: providerBaseUrlField

                        anchors.fill: parent
                        anchors.leftMargin: 10
                        anchors.rightMargin: 10
                        text: panel.providerFormBaseURL
                        color: panel.c("text_strong", "#f2f4f8")
                        font.family: panel.fontFamily
                        font.pixelSize: panel.fontSize("subtitle", 13)
                        verticalAlignment: TextInput.AlignVCenter
                        clip: true
                        onTextChanged: panel.providerFormBaseURL = text
                    }
                }
            }

            Item {
                width: parent.width
                height: 32

                Text {
                    width: panel.labelWidth
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "API key"
                    color: panel.c("text", "#d3d8e2")
                    font.family: panel.fontFamily
                    font.pixelSize: panel.fontSize("subtitle", 13)
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.leftMargin: panel.labelWidth + 8
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: 30
                    radius: panel.cornerRadius(7)
                    color: panel.c("input", "#0d1016")
                    border.width: 1
                    border.color: providerKeyField.activeFocus ? panel.c("primary", "#a7c7ff") : panel.c("panel_border", "#2b303b")

                    TextInput {
                        id: providerKeyField

                        anchors.fill: parent
                        anchors.leftMargin: 10
                        anchors.rightMargin: 10
                        text: panel.providerFormKey
                        echoMode: TextInput.Password
                        color: panel.c("text_strong", "#f2f4f8")
                        font.family: panel.fontFamily
                        font.pixelSize: panel.fontSize("subtitle", 13)
                        verticalAlignment: TextInput.AlignVCenter
                        clip: true
                        onTextChanged: panel.providerFormKey = text
                    }
                }
            }

            Item {
                width: parent.width
                height: 32

                Text {
                    width: panel.labelWidth
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Models"
                    color: panel.c("text", "#d3d8e2")
                    font.family: panel.fontFamily
                    font.pixelSize: panel.fontSize("subtitle", 13)
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.leftMargin: panel.labelWidth + 8
                    anchors.right: addFormModelButton.left
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    height: 30
                    radius: panel.cornerRadius(7)
                    color: panel.c("input", "#0d1016")
                    border.width: 1
                    border.color: providerModelInputField.activeFocus ? panel.c("primary", "#a7c7ff") : panel.c("panel_border", "#2b303b")

                    TextInput {
                        id: providerModelInputField

                        anchors.fill: parent
                        anchors.leftMargin: 10
                        anchors.rightMargin: 10
                        color: panel.c("text_strong", "#f2f4f8")
                        font.family: panel.fontFamily
                        font.pixelSize: panel.fontSize("subtitle", 13)
                        verticalAlignment: TextInput.AlignVCenter
                        clip: true
                        Keys.onReturnPressed: panel.addFormModelFromInput()
                        Keys.onEnterPressed: panel.addFormModelFromInput()
                    }
                }

                PaletteButton {
                    id: addFormModelButton

                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Add model"
                    tooltipText: "Add this model id to the provider"
                    theme: panel.theme
                    filled: true
                    primary: providerModelInputField.text.trim().length > 0
                    enabled: providerModelInputField.text.trim().length > 0 && !panel.providerAddBusy
                    onClicked: panel.addFormModelFromInput()
                }
            }

            Flow {
                width: parent.width
                spacing: 4
                leftPadding: panel.labelWidth + 8
                visible: panel.providerFormModels.length > 0

                Repeater {
                    model: panel.providerFormModels

                    delegate: Rectangle {
                        height: 22
                        width: formModelRow.implicitWidth + 20
                        radius: panel.cornerRadius(5)
                        color: panel.c("surface_elevated", "#1b1f28")
                        border.width: 1
                        border.color: panel.c("panel_border", "#2b303b")

                        Row {
                            id: formModelRow

                            anchors.verticalCenter: parent.verticalCenter
                            anchors.left: parent.left
                            anchors.leftMargin: 8
                            spacing: 6

                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(modelData)
                                color: panel.c("text", "#d3d8e2")
                                font.family: panel.fontFamily
                                font.pixelSize: panel.fontSize("body_small", 11)
                            }

                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "\u00d7"
                                color: panel.c("text_muted", "#8a93a3")
                                font.family: panel.fontFamily
                                font.pixelSize: panel.fontSize("body_small", 12)

                                MouseArea {
                                    anchors.fill: parent
                                    anchors.margins: -6
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: panel.removeFormModel(String(modelData))
                                }
                            }
                        }
                    }
                }
            }

            Item {
                width: parent.width
                height: 34

                PaletteButton {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: panel.providerAddBusy ? "Saving" : (panel.editingProviderID.length > 0 ? "Save" : "Add")
                    tooltipText: panel.editingProviderID.length > 0 ? "Save this provider" : "Add this provider"
                    theme: panel.theme
                    filled: true
                    primary: panel.providerFormLabel.trim().length > 0 && panel.providerFormBaseURL.trim().length > 0
                    enabled: panel.providerFormLabel.trim().length > 0 && panel.providerFormBaseURL.trim().length > 0 && !panel.providerAddBusy
                    onClicked: panel.providerSaveRequested(panel.editingProviderID, panel.providerFormLabel, panel.providerFormBaseURL, panel.providerFormKey, panel.providerFormModels)
                }
            }
        }

        Text {
            width: parent.width
            text: "API KEYS"
            color: panel.c("text_muted", "#8a93a3")
            font.family: panel.fontFamily
            font.pixelSize: panel.fontSize("caption", 10)
            font.letterSpacing: 1.2
            font.weight: Font.DemiBold
            topPadding: 14
            bottomPadding: 4
        }

        SecretSettingRow {
            id: openAISecretRow

            width: parent.width
            title: "OpenAI"
            labelWidth: panel.labelWidth
            configured: panel.secretConfigured
            source: panel.secretSource
            keyInput: panel.secretKeyInput
            saveBusy: panel.secretSaveBusy
            deleteBusy: panel.secretDeleteBusy
            theme: panel.theme
            fontFamily: panel.fontFamily
            onInputChanged: text => panel.secretInputChanged(text)
            onSaveRequested: panel.secretSaveRequested()
            onDeleteRequested: panel.secretDeleteRequested()
            onCancelRequested: panel.cancelRequested()
        }

        SecretSettingRow {
            id: openRouterSecretRow

            width: parent.width
            title: "OpenRouter"
            labelWidth: panel.labelWidth
            configured: panel.openRouterSecretConfigured
            source: panel.openRouterSecretSource
            keyInput: panel.openRouterSecretKeyInput
            saveBusy: panel.openRouterSecretSaveBusy
            deleteBusy: panel.openRouterSecretDeleteBusy
            theme: panel.theme
            fontFamily: panel.fontFamily
            onInputChanged: text => panel.openRouterSecretInputChanged(text)
            onSaveRequested: panel.openRouterSecretSaveRequested()
            onDeleteRequested: panel.openRouterSecretDeleteRequested()
            onCancelRequested: panel.cancelRequested()
        }

        SecretSettingRow {
            id: xAISecretRow

            width: parent.width
            title: "xAI"
            labelWidth: panel.labelWidth
            configured: panel.xAISecretConfigured
            source: panel.xAISecretSource
            keyInput: panel.xAISecretKeyInput
            saveBusy: panel.xAISecretSaveBusy
            deleteBusy: panel.xAISecretDeleteBusy
            theme: panel.theme
            fontFamily: panel.fontFamily
            onInputChanged: text => panel.xAISecretInputChanged(text)
            onSaveRequested: panel.xAISecretSaveRequested()
            onDeleteRequested: panel.xAISecretDeleteRequested()
            onCancelRequested: panel.cancelRequested()
        }

    }
}
