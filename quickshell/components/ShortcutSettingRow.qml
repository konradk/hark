import QtQuick

Item {
    id: control

    property string title: ""
    property string hint: ""
    property real labelWidth: 150
    property string shortcut: ""
    property bool configured: false
    property bool recording: false
    property bool busy: false
    property var theme: null

    signal recordRequested()
    signal disableRequested()
    signal shortcutKeyPressed(var event)

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

    function beginRecording() {
        recorder.forceActiveFocus();
    }

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    implicitHeight: hint.length > 0 ? 38 : 34

    Row {
        anchors.fill: parent
        spacing: 8

        Column {
            id: shortcutLabel

            width: control.labelWidth
            spacing: 2
            anchors.verticalCenter: parent.verticalCenter

            Text {
                width: parent.width
                text: control.title
                color: control.c("text", "#d3d8e2")
                font.family: control.fontFamily
                font.pixelSize: control.fontSize("subtitle", 13)
                elide: Text.ElideRight
            }

            Text {
                width: parent.width
                visible: control.hint.length > 0
                text: control.hint
                color: control.c("text_muted", "#8a93a3")
                font.family: control.fontFamily
                font.pixelSize: control.fontSize("body_small", 11)
                elide: Text.ElideRight
            }

        }

        Rectangle {
            id: recorder

            width: parent.width - shortcutLabel.width - parent.spacing - (disableButton.visible ? disableButton.width + parent.spacing : 0)
            height: 32
            anchors.verticalCenter: parent.verticalCenter
            radius: control.cornerRadius(7)
            color: control.recording ? control.c("surface_active", "#2a3040") : recorderMouse.containsMouse ? control.c("surface_hover", "#222733") : control.c("input", "#0d1016")
            border.width: control.recording || activeFocus ? 1 : 0
            border.color: control.c("primary", "#a7c7ff")
            activeFocusOnTab: true
            Keys.onPressed: (event) => {
                if (!control.recording)
                    return ;

                event.accepted = true;
                control.shortcutKeyPressed(event);
            }

            Text {
                anchors.left: parent.left
                anchors.leftMargin: 10
                anchors.right: parent.right
                anchors.rightMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                text: control.recording ? "Press shortcut..." : control.configured ? control.shortcut : "Click to set a shortcut"
                color: control.recording ? control.c("text_strong", "#f2f4f8") : control.configured ? control.c("text", "#d3d8e2") : control.c("text_muted", "#8a93a3")
                font.family: control.fontFamily
                font.pixelSize: control.fontSize("body", 12)
                elide: Text.ElideRight
            }

            MouseArea {
                id: recorderMouse

                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: control.recordRequested()
            }

        }

        PaletteButton {
            id: disableButton

            width: 64
            height: 32
            anchors.verticalCenter: parent.verticalCenter
            visible: control.configured
            text: "Disable"
            tooltipText: "Remove shortcut"
            danger: true
            filled: true
            theme: control.theme
            enabled: !control.busy
            onClicked: control.disableRequested()
        }

    }

}
