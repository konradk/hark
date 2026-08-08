import QtQuick

Item {
    id: hint

    property var theme: null
    property bool apiKeyConfigured: true

    signal suggestionPicked(string text)
    signal screenshotRequested()
    signal apiKeySetupRequested()

    function c(name, fallback) {
        return theme && theme[name] ? theme[name] : fallback;
    }

    function fontSize(role, fallback) {
        const configured = theme ? Number(theme["font_" + role]) : NaN;
        return isFinite(configured) && configured > 0 ? configured : fallback;
    }

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    Column {
        anchors.centerIn: parent
        width: Math.min(parent.width, 520)
        spacing: 16

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: hint.apiKeyConfigured ? "What do you want to know?" : "Add an API key to get started"
            color: hint.c("text_muted", "#8a93a3")
            font.family: hint.fontFamily
            font.pixelSize: hint.fontSize("title", 14)
        }

        Text {
            width: parent.width
            visible: !hint.apiKeyConfigured
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
            text: "Hark needs an OpenAI or OpenRouter API key to answer questions."
            color: hint.c("text_muted", "#8a93a3")
            font.family: hint.fontFamily
            font.pixelSize: hint.fontSize("body", 12)
        }

        PaletteButton {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: !hint.apiKeyConfigured
            text: "Set up API key"
            filled: true
            primary: true
            tooltipText: "Add API key"
            theme: hint.theme
            onClicked: hint.apiKeySetupRequested()
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 8
            visible: hint.apiKeyConfigured

            PaletteButton {
                text: "Explain this error"
                filled: true
                tooltipText: "Use starter prompt"
                theme: hint.theme
                onClicked: hint.suggestionPicked(text)
            }

            PaletteButton {
                text: "Rewrite clearly"
                filled: true
                tooltipText: "Use starter prompt"
                theme: hint.theme
                onClicked: hint.suggestionPicked(text)
            }

            PaletteButton {
                text: "Compare options"
                filled: true
                tooltipText: "Use starter prompt"
                theme: hint.theme
                onClicked: hint.suggestionPicked(text)
            }

        }

        PaletteButton {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: hint.apiKeyConfigured
            text: "Ask about a screenshot"
            symbol: "paperclip"
            filled: true
            hint: "⌃⇧C"
            tooltipText: "Attach screenshot region · Ctrl+Shift+C"
            theme: hint.theme
            onClicked: hint.screenshotRequested()
        }

    }

}
