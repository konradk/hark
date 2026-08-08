import QtQuick

Item {
    id: control

    property string title: ""
    property string placeholder: "Paste API key..."
    property real labelWidth: 150
    property bool configured: false
    property string source: "none"
    property string keyInput: ""
    property bool saveBusy: false
    property bool deleteBusy: false
    property bool editing: false
    property var theme: null
    property string fontFamily: ""

    signal inputChanged(string text)
    signal saveRequested()
    signal deleteRequested()
    signal cancelRequested()

    readonly property bool storedInKeyring: configured && source !== "environment"
    readonly property string statusText: source === "environment" ? "Set by environment variable" : configured ? "Saved in keyring" : "Not set"

    function c(name, fallback) {
        return theme && theme[name] ? theme[name] : fallback;
    }

    function cornerRadius(maximum) {
        const configuredRadius = theme && theme.corner_radius !== undefined ? Number(theme.corner_radius) : maximum;
        return Math.min(Math.max(0, configuredRadius), maximum);
    }

    function fontSize(role, fallback) {
        const configuredSize = theme ? Number(theme["font_" + role]) : NaN;
        return isFinite(configuredSize) && configuredSize > 0 ? configuredSize : fallback;
    }

    function beginEditing() {
        keyInputField.text = "";
        editing = true;
        keyInputField.forceActiveFocus();
    }

    function stopEditing() {
        keyInputField.text = "";
        editing = false;
    }

    function focusInput() {
        beginEditing();
    }

    implicitHeight: 34
    onSaveBusyChanged: {
        if (!saveBusy && control.keyInput.length === 0)
            editing = false;

    }

    Text {
        id: label

        width: control.labelWidth
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: control.title
        color: control.c("text", "#d3d8e2")
        font.family: control.fontFamily
        font.pixelSize: control.fontSize("subtitle", 13)
        elide: Text.ElideRight
    }

    Row {
        id: actions

        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: 8

        PaletteButton {
            visible: !control.editing
            text: control.configured ? "Replace" : "Add key"
            tooltipText: control.configured ? "Replace stored key" : "Add API key"
            theme: control.theme
            filled: true
            primary: !control.configured
            onClicked: control.beginEditing()
        }

        PaletteButton {
            visible: !control.editing && control.storedInKeyring
            text: "Delete"
            tooltipText: "Delete stored key"
            danger: true
            filled: true
            theme: control.theme
            enabled: !control.deleteBusy
            onClicked: control.deleteRequested()
        }

        PaletteButton {
            visible: control.editing
            text: control.saveBusy ? "Saving" : "Save"
            tooltipText: "Save API key"
            theme: control.theme
            filled: true
            primary: control.keyInput.trim().length > 0
            enabled: control.keyInput.trim().length > 0 && !control.saveBusy
            onClicked: control.saveRequested()
        }

        PaletteButton {
            visible: control.editing
            text: "Cancel"
            tooltipText: "Discard key"
            theme: control.theme
            enabled: !control.saveBusy
            onClicked: control.stopEditing()
        }

    }

    Text {
        visible: !control.editing
        anchors.left: label.right
        anchors.leftMargin: 8
        anchors.right: actions.left
        anchors.rightMargin: 12
        anchors.verticalCenter: parent.verticalCenter
        text: control.statusText
        color: control.configured ? control.c("text_muted", "#8a93a3") : control.c("text_disabled", "#5b6373")
        font.family: control.fontFamily
        font.pixelSize: control.fontSize("body", 12)
        elide: Text.ElideRight
    }

    Rectangle {
        visible: control.editing
        anchors.left: label.right
        anchors.leftMargin: 8
        anchors.right: actions.left
        anchors.rightMargin: 8
        anchors.verticalCenter: parent.verticalCenter
        height: 32
        radius: control.cornerRadius(7)
        color: control.c("input", "#0d1016")
        border.width: 1
        border.color: keyInputField.activeFocus ? control.c("primary", "#a7c7ff") : control.c("panel_border", "#2b303b")

        TextInput {
            id: keyInputField

            anchors.fill: parent
            anchors.leftMargin: 10
            anchors.rightMargin: 10
            text: control.keyInput
            echoMode: TextInput.Password
            color: control.c("text_strong", "#f2f4f8")
            selectedTextColor: control.c("selection_text", "#10131a")
            selectionColor: control.c("primary", "#a7c7ff")
            font.family: control.fontFamily
            font.pixelSize: control.fontSize("subtitle", 13)
            verticalAlignment: TextInput.AlignVCenter
            clip: true
            onTextChanged: control.inputChanged(text)
            Keys.onReturnPressed: control.saveRequested()
            Keys.onEnterPressed: control.saveRequested()
            Keys.onEscapePressed: {
                if (control.configured || control.keyInput.length > 0)
                    control.stopEditing();
                else
                    control.cancelRequested();
            }

            Text {
                anchors.fill: parent
                visible: keyInputField.text.length === 0
                text: control.placeholder
                color: control.c("text_disabled", "#5b6373")
                font.family: control.fontFamily
                font.pixelSize: keyInputField.font.pixelSize
                verticalAlignment: Text.AlignVCenter
            }

        }

    }

}
