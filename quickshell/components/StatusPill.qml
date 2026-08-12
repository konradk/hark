import QtQuick
import QtQuick.Controls

Item {
    id: pill

    property string statusText: ""
    property bool danger: false
    property var theme: null

    function c(name, fallback) {
        return theme && theme[name] ? theme[name] : fallback;
    }

    function fontSize(role, fallback) {
        const configured = theme ? Number(theme["font_" + role]) : NaN;
        return isFinite(configured) && configured > 0 ? configured : fallback;
    }

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    implicitHeight: 32
    visible: width > 8
    ToolTip.text: statusText
    ToolTip.visible: statusHover.hovered && label.truncated
    ToolTip.delay: 500

    HoverHandler {
        id: statusHover
    }

    Row {
        anchors.fill: parent
        spacing: 7

        Rectangle {
            id: statusDot

            width: 7
            height: 7
            radius: 4
            anchors.verticalCenter: parent.verticalCenter
            visible: label.text.length > 0
            color: pill.danger ? pill.c("error", "#d66a6a") : pill.c("text_muted", "#7f8794")

        }

        Text {
            id: label

            width: Math.max(0, parent.width - (text.length > 0 ? 14 : 0))
            anchors.verticalCenter: parent.verticalCenter
            text: pill.statusText
            color: pill.danger ? pill.c("error_text", "#ffb3b3") : pill.c("text_muted", "#7f8794")
            font.family: pill.fontFamily
            font.pixelSize: pill.fontSize("body", 13)
            elide: Text.ElideRight
        }

    }

}
