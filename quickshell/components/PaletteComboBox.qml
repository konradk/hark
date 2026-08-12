import QtQuick
import QtQuick.Controls

ComboBox {
    id: control

    property string tooltipText: ""
    property var theme: null
    property real popupMinimumWidth: 150
    property real popupMaximumWidth: 360

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

    function preferredPopupWidth() {
        let widestLabel = 0;
        for (let index = 0; index < count; index++)
            widestLabel = Math.max(widestLabel, optionFontMetrics.advanceWidth(textAt(index)));
        return Math.min(popupMaximumWidth, Math.max(width, popupMinimumWidth, Math.ceil(widestLabel) + 40));
    }

    readonly property string fontFamily: String(theme && theme.font_family !== undefined ? theme.font_family : Qt.application.font.family)

    implicitHeight: 28
    leftPadding: 9
    rightPadding: 24
    focusPolicy: Qt.TabFocus
    ToolTip.text: selectedLabel.truncated && displayText.length > 0 ? displayText + (tooltipText.length > 0 ? "\n" + tooltipText : "") : tooltipText
    ToolTip.visible: ToolTip.text.length > 0 && hovered
    ToolTip.delay: 500

    FontMetrics {
        id: optionFontMetrics

        font.family: control.fontFamily
        font.pixelSize: control.fontSize("body", 12)
    }

    contentItem: Text {
        id: selectedLabel

        text: control.displayText
        color: !control.enabled ? control.c("text_disabled", "#5b6373") : control.hovered || control.popup.visible ? control.c("text_strong", "#f2f4f8") : control.c("text", "#d3d8e2")
        font.family: control.fontFamily
        font.pixelSize: control.fontSize("body", 12)
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    indicator: IconMark {
        anchors.right: parent.right
        anchors.rightMargin: 7
        anchors.verticalCenter: parent.verticalCenter
        width: 12
        height: 12
        symbol: "chevron-down"
        color: control.enabled ? control.c("text_muted", "#8a93a3") : control.c("text_disabled", "#5b6373")
    }

    background: Rectangle {
        radius: control.cornerRadius(7)
        color: !control.enabled ? "transparent" : control.down || control.popup.visible ? control.c("button_down", "#2d3340") : control.hovered ? control.c("button_hover", "#252a35") : "transparent"
        border.width: control.activeFocus ? 1 : 0
        border.color: control.c("primary", "#a7c7ff")

        Behavior on color {
            ColorAnimation {
                duration: 100
            }

        }

    }

    delegate: ItemDelegate {
        id: option

        required property string label
        required property int index

        width: ListView.view ? ListView.view.width : control.popup.availableWidth
        height: 32
        highlighted: control.highlightedIndex === index

        contentItem: Text {
            leftPadding: 6
            text: option.label
            color: option.highlighted ? control.c("text_strong", "#f2f4f8") : control.c("text", "#d3d8e2")
            font.family: control.fontFamily
            font.pixelSize: control.fontSize("body", 12)
            verticalAlignment: Text.AlignVCenter
            elide: Text.ElideRight
        }

        background: Rectangle {
            radius: control.cornerRadius(5)
            color: option.highlighted ? control.c("surface_active", "#2a3040") : "transparent"
        }

    }

    popup: Popup {
        x: control.width - width
        y: -implicitHeight - 8
        width: control.preferredPopupWidth()
        implicitHeight: Math.min(contentItem.implicitHeight + 10, 240)
        padding: 5

        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: control.popup.visible ? control.delegateModel : null
            currentIndex: control.highlightedIndex
        }

        background: Rectangle {
            radius: control.cornerRadius(9)
            color: control.c("surface_elevated", "#1b1f27")
            border.width: 1
            border.color: control.c("panel_border", "#2c313c")
        }

    }

}
