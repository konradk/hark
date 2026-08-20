import QtQuick

Rectangle {
    id: row

    property string promptText: ""
    property string responseText: ""
    property int historyId: 0
    property bool selected: false
    property bool deleting: false
    property var theme: null

    signal entered()
    signal openRequested(int entryId)
    signal deleteRequested(int entryId)

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

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    function previewText(value) {
        return String(value ?? "").replace(/```[^`]*```/g, " ").replace(/`([^`]*)`/g, "$1").replace(/\[([^\]]+)\]\([^)]*\)/g, "$1").replace(/\*\*([^*]+)\*\*/g, "$1").replace(/__([^_]+)__/g, "$1").replace(/\*([^*\n]+)\*/g, "$1").replace(/~~([^~]+)~~/g, "$1").replace(/(^|\n)#{1,6}\s+/g, "$1").replace(/(^|\n)\s*>\s?/g, "$1").replace(/\s+/g, " ").trim();
    }

    height: 32
    radius: row.cornerRadius(7)
    color: selected ? row.c("surface_active", "#2a3040") : mouseArea.containsMouse ? row.c("surface_hover", "#222631") : "transparent"

    Row {
        anchors.left: parent.left
        anchors.right: deleteButton.left
        anchors.leftMargin: 10
        anchors.rightMargin: 8
        anchors.verticalCenter: parent.verticalCenter
        spacing: 10

        Text {
            objectName: "historyPromptPreview"
            text: row.previewText(row.promptText)
            color: row.selected || mouseArea.containsMouse ? row.c("text_strong", "#f2f4f8") : row.c("text", "#d3d8e2")
            font.family: row.fontFamily
            font.pixelSize: row.fontSize("body", 13)
            elide: Text.ElideRight
            maximumLineCount: 1
            width: Math.min(implicitWidth, parent.width * 0.62)
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            objectName: "historyResponsePreview"
            text: row.previewText(row.responseText)
            color: row.c("text_muted", "#8a93a3")
            font.family: row.fontFamily
            font.pixelSize: row.fontSize("body_small", 12)
            elide: Text.ElideRight
            maximumLineCount: 1
            width: parent.width - x
            anchors.verticalCenter: parent.verticalCenter
        }

    }

    MouseArea {
        id: mouseArea

        anchors.left: parent.left
        anchors.right: deleteButton.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        hoverEnabled: true
        onEntered: row.entered()
        onClicked: row.openRequested(row.historyId)
    }

    IconButton {
        id: deleteButton

        anchors.right: parent.right
        anchors.rightMargin: 3
        anchors.verticalCenter: parent.verticalCenter
        size: 24
        symbol: "close"
        tooltipText: "Delete chat"
        danger: true
        theme: row.theme
        enabled: !row.deleting
        opacity: mouseArea.containsMouse || row.selected || hovered || activeFocus ? 1 : 0
        onClicked: row.deleteRequested(row.historyId)

        Behavior on opacity {
            NumberAnimation {
                duration: 90
            }

        }

    }

    Behavior on color {
        ColorAnimation {
            duration: 100
        }

    }

}
