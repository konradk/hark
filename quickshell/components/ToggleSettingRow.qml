import QtQuick

Item {
    id: control

    property string title: ""
    property string hint: ""
    property bool checked: false
    property bool busy: false
    property var theme: null

    signal toggled(bool checked)

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

    function toggle() {
        if (enabled && !busy)
            toggled(!checked);

    }

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    implicitHeight: hint.length > 0 ? 38 : 32
    opacity: enabled && !busy ? 1 : 0.55
    activeFocusOnTab: true
    Keys.onSpacePressed: toggle()
    Keys.onReturnPressed: toggle()
    Keys.onEnterPressed: toggle()

    Column {
        anchors.left: parent.left
        anchors.right: toggleTrack.left
        anchors.rightMargin: 14
        anchors.verticalCenter: parent.verticalCenter
        spacing: 2

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
            height: visible ? implicitHeight : 0
            text: control.hint
            color: control.c("text_muted", "#8a93a3")
            font.family: control.fontFamily
            font.pixelSize: control.fontSize("body_small", 11)
            elide: Text.ElideRight
        }

    }

    Rectangle {
        id: toggleTrack

        width: 38
        height: 20
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        radius: control.cornerRadius(height / 2)
        color: control.checked ? control.c("primary", "#a7c7ff") : control.c("surface_active", "#2a3040")
        border.width: control.activeFocus ? 1 : 0
        border.color: control.c("text_strong", "#f2f4f8")

        Rectangle {
            width: 14
            height: 14
            y: 3
            x: control.checked ? parent.width - width - 3 : 3
            radius: control.cornerRadius(width / 2)
            color: control.checked ? control.c("primary_text", "#0b1220") : control.c("text_muted", "#8a93a3")

            Behavior on x {
                NumberAnimation {
                    duration: 110
                }

            }

        }

        Behavior on color {
            ColorAnimation {
                duration: 110
            }

        }

    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        enabled: control.enabled && !control.busy
        onClicked: {
            control.forceActiveFocus();
            control.toggle();
        }
    }

}
