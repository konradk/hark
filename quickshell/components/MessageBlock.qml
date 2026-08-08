import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../js/Markdown.js" as Markdown
import "../js/UrlSafety.js" as UrlSafety

Column {
    id: block

    property string role: "assistant"
    property string content: ""
    property bool pending: false
    property bool assistant: role === "assistant"
    property bool errorRole: role === "error"
    property bool userRole: !assistant && !errorRole
    property var theme: null
    property var contentSegments: []
    readonly property bool showMessageCopy: headerMouse.containsMouse || bodyMouse.containsMouse || copyMessageButton.activeFocus || copyMessageButton.hovered

    signal copyRequested(string text)

    // Re-parsing the whole message on every streamed token is quadratic, so a
    // pending message coalesces parses onto a timer. The finished message is
    // always parsed immediately.
    function refreshSegments() {
        contentSegments = Markdown.parseContent(content, assistant, errorRole);
    }

    onContentChanged: {
        if (block.pending) {
            parseThrottle.restart();
        } else {
            parseThrottle.stop();
            block.refreshSegments();
        }
    }
    onPendingChanged: {
        parseThrottle.stop();
        block.refreshSegments();
    }
    onRoleChanged: {
        parseThrottle.stop();
        block.refreshSegments();
    }
    Component.onCompleted: block.refreshSegments()

    Timer {
        id: parseThrottle

        interval: 90
        repeat: false
        onTriggered: block.refreshSegments()
    }

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

    function openExternalUrl(value) {
        const safeUrl = UrlSafety.safeExternalUrl(value);
        if (safeUrl.length === 0)
            return false;

        return Qt.openUrlExternally(safeUrl);
    }

    function sourcesHtml(items) {
        return Markdown.sourcesHtml(items, theme);
    }

    function inlineMarkdownToHtml(value) {
        return Markdown.inlineMarkdownToHtml(value, theme);
    }

    function markdownBlockToHtml(value) {
        return Markdown.markdownBlockToHtml(value, theme);
    }

    spacing: 6

    Item {
        width: parent.width
        height: 18

        MouseArea {
            id: headerMouse

            anchors.fill: parent
            acceptedButtons: Qt.NoButton
            hoverEnabled: true
        }

        Row {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: 8

            Text {
                text: block.errorRole ? "ERROR" : block.assistant ? "ANSWER" : "YOU"
                color: block.errorRole ? block.c("error_text", "#ffb4b4") : block.c("text_muted", "#8a93a3")
                font.family: block.fontFamily
                font.pixelSize: block.fontSize("caption", 10)
                font.letterSpacing: 1.2
                font.weight: Font.DemiBold
                anchors.verticalCenter: parent.verticalCenter
            }

            Row {
                visible: block.pending && block.content.length === 0
                spacing: 4
                anchors.verticalCenter: parent.verticalCenter

                Repeater {
                    model: 3

                    Rectangle {
                        required property int index

                        width: 5
                        height: 5
                        radius: 2.5
                        anchors.verticalCenter: parent.verticalCenter
                        color: block.c("text_muted", "#8a93a3")
                        opacity: 0.25

                        SequentialAnimation on opacity {
                            running: block.pending && block.content.length === 0
                            loops: 1

                            PauseAnimation {
                                duration: index * 180
                            }

                            SequentialAnimation {
                                loops: Animation.Infinite

                                NumberAnimation {
                                    from: 0.25
                                    to: 1
                                    duration: 380
                                    easing.type: Easing.InOutSine
                                }

                                NumberAnimation {
                                    from: 1
                                    to: 0.25
                                    duration: 380
                                    easing.type: Easing.InOutSine
                                }

                                PauseAnimation {
                                    duration: 180
                                }

                            }

                        }

                    }

                }

            }

        }

        IconButton {
            id: copyMessageButton

            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            size: 22
            symbol: "copy"
            tooltipText: "Copy message"
            theme: block.theme
            enabled: block.content.length > 0
            opacity: block.showMessageCopy ? 1 : 0
            onClicked: block.copyRequested(block.content)

            Behavior on opacity {
                NumberAnimation {
                    duration: 90
                }

            }

        }

    }

    Rectangle {
        width: parent.width
        implicitHeight: contentColumn.implicitHeight + (block.errorRole ? 20 : 4)
        radius: block.cornerRadius(8)
        color: block.errorRole ? block.c("error_surface", "#251a1d") : "transparent"
        border.width: block.errorRole ? 1 : 0
        border.color: block.c("error_border", "#5d3338")

        MouseArea {
            id: bodyMouse

            anchors.fill: parent
            acceptedButtons: Qt.NoButton
            hoverEnabled: true
        }

        Column {
            id: contentColumn

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.leftMargin: block.errorRole ? 12 : 0
            anchors.rightMargin: block.errorRole ? 12 : 0
            anchors.topMargin: block.errorRole ? 10 : 0
            spacing: 8

            Repeater {
                // Count-only model keeps existing loaders alive.
                model: block.contentSegments.length

                Loader {
                    property var segment: block.contentSegments[index] ?? ({
                    })
                    property bool firstSegment: index === 0

                    width: contentColumn.width
                    sourceComponent: {
                        switch (segment.kind) {
                        case "code":
                            return codeSegment;
                        case "heading":
                            return headingSegment;
                        case "quote":
                            return quoteSegment;
                        case "table":
                            return tableSegment;
                        case "sources":
                            return sourcesSegment;
                        case "hr":
                            return hrSegment;
                        default:
                            return markdownSegment;
                        }
                    }
                }

            }

        }

    }

    Component {
        id: sourcesSegment

        Column {
            width: parent ? parent.width : 0
            spacing: 7
            topPadding: 8

            Rectangle {
                width: parent.width
                height: 1
                color: block.c("text_disabled", "#5b6373")
                opacity: 0.35
            }

            Text {
                text: "SOURCES"
                color: block.c("text_disabled", "#5b6373")
                font.family: block.fontFamily
                font.pixelSize: block.fontSize("caption", 10)
                font.letterSpacing: 1.2
                font.weight: Font.DemiBold
            }

            Text {
                width: parent.width
                text: block.sourcesHtml(segment.items)
                textFormat: Text.RichText
                wrapMode: Text.Wrap
                color: block.c("text_muted", "#8a93a3")
                linkColor: block.c("text_muted", "#8a93a3")
                lineHeight: 1.25
                font.family: block.fontFamily
                font.pixelSize: block.fontSize("body_small", 13)
                onLinkActivated: (link) => {
                    return block.openExternalUrl(link);
                }

                HoverHandler {
                    enabled: parent.hoveredLink.length > 0
                    cursorShape: Qt.PointingHandCursor
                }

            }

        }

    }

    Component {
        id: markdownSegment

        Text {
            width: parent ? parent.width : 0
            text: block.markdownBlockToHtml(segment.text)
            textFormat: Text.RichText
            wrapMode: Text.Wrap
            color: block.errorRole ? block.c("error_text", "#ffd3d3") : block.userRole ? block.c("text_muted", "#8a93a3") : block.c("text", "#d3d8e2")
            linkColor: block.c("text_strong", "#f2f4f8")
            lineHeight: 1.3
            font.family: block.fontFamily
            font.pixelSize: block.assistant ? block.fontSize("subtitle", 15) : block.fontSize("body", 14)
            onLinkActivated: (link) => {
                return block.openExternalUrl(link);
            }

            HoverHandler {
                enabled: parent.hoveredLink.length > 0
                cursorShape: Qt.PointingHandCursor
            }

        }

    }

    Component {
        id: headingSegment

        Text {
            width: parent ? parent.width : 0
            text: block.inlineMarkdownToHtml(segment.text)
            textFormat: Text.RichText
            wrapMode: Text.Wrap
            color: block.c("text_strong", "#f2f4f8")
            linkColor: block.c("text_strong", "#f2f4f8")
            lineHeight: 1.2
            topPadding: firstSegment ? 0 : (segment.level <= 2 ? 10 : 6)
            bottomPadding: 2
            font.family: block.fontFamily
            font.pixelSize: segment.level <= 1 ? block.fontSize("icon_large", 19) : segment.level === 2 ? block.fontSize("heading", 17) : segment.level === 3 ? block.fontSize("title", 16) : block.fontSize("subtitle", 15)
            font.weight: Font.DemiBold
        }

    }

    Component {
        id: quoteSegment

        Item {
            implicitHeight: quoteText.implicitHeight + 8

            Rectangle {
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.topMargin: 2
                anchors.bottomMargin: 2
                width: 3
                radius: 1.5
                color: block.c("assistant", "#8fbce8")
                opacity: 0.65
            }

            Text {
                id: quoteText

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.leftMargin: 14
                anchors.verticalCenter: parent.verticalCenter
                text: block.inlineMarkdownToHtml(segment.text).replace(/\n/g, "<br/>")
                textFormat: Text.RichText
                wrapMode: Text.Wrap
                color: block.c("text_muted", "#8a93a3")
                linkColor: block.c("text_strong", "#f2f4f8")
                lineHeight: 1.3
                font.family: block.fontFamily
                font.pixelSize: block.fontSize("body", 14)
                onLinkActivated: (link) => {
                    return block.openExternalUrl(link);
                }
            }

        }

    }

    Component {
        id: tableSegment

        GridLayout {
            columns: segment.columns
            columnSpacing: 18
            rowSpacing: 5

            Repeater {
                model: segment.cells

                Loader {
                    required property var modelData

                    Layout.fillWidth: true
                    Layout.columnSpan: modelData.cellKind === "sep" ? segment.columns : 1
                    sourceComponent: modelData.cellKind === "sep" ? tableRule : tableCell

                    Component {
                        id: tableCell

                        Text {
                            width: parent ? parent.width : 0
                            text: block.inlineMarkdownToHtml(modelData.text)
                            textFormat: Text.RichText
                            wrapMode: Text.Wrap
                            color: modelData.header ? block.c("text_strong", "#f2f4f8") : block.c("text", "#d3d8e2")
                            linkColor: block.c("text_strong", "#f2f4f8")
                            lineHeight: 1.25
                            font.family: block.fontFamily
                            font.pixelSize: block.fontSize("body", 14)
                            font.weight: modelData.header ? Font.DemiBold : Font.Normal
                        }

                    }

                    Component {
                        id: tableRule

                        Item {
                            implicitHeight: 1

                            Rectangle {
                                width: parent.width
                                height: 1
                                color: block.c("text_disabled", "#5b6373")
                                opacity: 0.5
                            }

                        }

                    }

                }

            }

        }

    }

    Component {
        id: hrSegment

        Item {
            implicitHeight: 13

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                height: 1
                color: block.c("text_disabled", "#5b6373")
                opacity: 0.5
            }

        }

    }

    Component {
        id: codeSegment

        Rectangle {
            width: parent ? parent.width : 0
            implicitHeight: codeHeader.height + codeText.implicitHeight + 20
            radius: block.cornerRadius(8)
            color: block.c("surface", "#11141a")
            border.width: 1
            border.color: block.c("panel_border", "#2c313c")

            MouseArea {
                id: codeMouse

                anchors.fill: parent
                acceptedButtons: Qt.NoButton
                hoverEnabled: true
            }

            Row {
                id: codeHeader

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.leftMargin: 12
                anchors.rightMargin: 6
                anchors.topMargin: 6
                height: 24
                spacing: 8

                Text {
                    width: parent.width - copyCodeButton.width - parent.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(segment.language ?? "").length > 0 ? String(segment.language).toUpperCase() : "CODE"
                    color: block.c("text_muted", "#8a93a3")
                    font.family: block.fontFamily
                    font.pixelSize: block.fontSize("caption", 10)
                    font.letterSpacing: 1.2
                    elide: Text.ElideRight
                }

                IconButton {
                    id: copyCodeButton

                    size: 22
                    symbol: "copy"
                    tooltipText: "Copy code"
                    theme: block.theme
                    enabled: String(segment.text ?? "").length > 0
                    opacity: codeMouse.containsMouse || hovered || activeFocus ? 1 : 0
                    onClicked: block.copyRequested(String(segment.text ?? ""))

                    Behavior on opacity {
                        NumberAnimation {
                            duration: 90
                        }

                    }

                }

            }

            TextArea {
                id: codeText

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: codeHeader.bottom
                anchors.leftMargin: 12
                anchors.rightMargin: 12
                anchors.topMargin: 2
                text: String(segment.text ?? "")
                readOnly: true
                selectByMouse: true
                textFormat: TextEdit.PlainText
                wrapMode: TextEdit.Wrap
                color: block.c("text_strong", "#f2f4f8")
                selectedTextColor: block.c("selection_text", "#10131a")
                selectionColor: block.c("primary", "#a7c7ff")
                font.family: block.fontFamily
                font.pixelSize: block.fontSize("body", 13)
                padding: 0

                background: Rectangle {
                    color: "transparent"
                }

            }

        }

    }

}
