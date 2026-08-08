import QtQuick
import QtQuick.Controls

Button {
    id: control

    property bool primary: false
    property bool danger: false
    property bool filled: false
    property string tooltipText: ""
    property string symbol: ""
    property string hint: ""
    property var theme: null
    readonly property bool hasIcon: symbol.length > 0
    readonly property bool hasLabel: text.length > 0
    readonly property bool hasHint: hint.length > 0
    readonly property color fgColor: !enabled ? c("text_disabled", "#5b6373") : danger ? c("error_text", "#ffb4b4") : primary ? c("primary_text", "#0b1220") : hovered || down ? c("text_strong", "#f2f4f8") : c("text", "#d3d8e2")

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

    implicitWidth: hasIcon && !hasLabel ? 30 : Math.ceil(labelMetrics.advanceWidth) + (hasIcon ? 23 : 0) + (hasHint ? Math.ceil(hintMetrics.advanceWidth) + 18 : 0) + leftPadding + rightPadding
    implicitHeight: 30
    leftPadding: hasIcon && !hasLabel ? 0 : 11
    rightPadding: hasIcon && !hasLabel ? 0 : 11
    focusPolicy: Qt.TabFocus
    ToolTip.text: tooltipText
    ToolTip.visible: tooltipText.length > 0 && hovered
    ToolTip.delay: 500

    TextMetrics {
        id: labelMetrics

        text: control.text
        font.family: control.fontFamily
        font.pixelSize: control.fontSize("body", 13)
    }

    TextMetrics {
        id: hintMetrics

        text: control.hint
        font.family: control.fontFamily
        font.pixelSize: control.fontSize("body_small", 11)
    }

    contentItem: Item {
        Row {
            anchors.centerIn: parent
            spacing: 7

            IconMark {
                anchors.verticalCenter: parent.verticalCenter
                visible: control.hasIcon
                symbol: control.symbol
                color: control.fgColor
            }

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: control.text
                visible: control.hasLabel
                color: control.fgColor
                font.family: control.fontFamily
                font.pixelSize: control.fontSize("body", 13)
                elide: Text.ElideRight
            }

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                visible: control.hasHint
                width: Math.ceil(hintMetrics.advanceWidth) + 10
                height: 18
                radius: control.cornerRadius(4)
                color: control.primary ? Qt.rgba(0, 0, 0, 0.18) : control.c("surface_active", "#2a3040")

                Text {
                    anchors.centerIn: parent
                    text: control.hint
                    color: control.primary ? control.fgColor : control.c("text_muted", "#8a93a3")
                    font.family: control.fontFamily
                    font.pixelSize: control.fontSize("body_small", 11)
                }

            }

        }

    }

    background: Rectangle {
        radius: control.cornerRadius(7)
        color: {
            if (!control.enabled)
                return control.primary || control.filled ? control.c("button_disabled", "#171a21") : "transparent";

            if (control.down)
                return control.danger ? control.c("error_surface", "#251a1d") : control.primary ? control.c("primary_down", "#8fb6f2") : control.c("button_down", "#2d3340");

            if (control.hovered)
                return control.danger ? control.c("error_surface", "#251a1d") : control.primary ? control.c("primary_hover", "#b8d2ff") : control.c("button_hover", "#252a35");

            return control.primary ? control.c("primary", "#a7c7ff") : control.filled ? control.c("button", "#1d212b") : "transparent";
        }
        border.width: control.activeFocus ? 1 : 0
        border.color: control.c("primary", "#a7c7ff")

        Behavior on color {
            ColorAnimation {
                duration: 100
            }

        }

    }

}
