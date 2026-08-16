import QtQuick

Item {
    id: control

    property string providerId: ""
    property string label: ""
    property string baseUrl: ""
    property var models: []
    property bool busy: false
    property var theme: null
    property string fontFamily: ""

    signal editRequested()
    signal removeRequested()

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

    implicitHeight: contentColumn.implicitHeight

    Column {
        id: contentColumn

        width: parent.width
        spacing: 3

        Row {
            width: parent.width
            height: 26

            Text {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, parent.width - 2 * 68 - 16)
                text: control.label
                color: control.c("text_strong", "#f2f4f8")
                font.family: control.fontFamily
                font.pixelSize: control.fontSize("subtitle", 13)
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }

            PaletteButton {
                anchors.verticalCenter: parent.verticalCenter
                text: "Edit"
                tooltipText: "Edit this provider"
                theme: control.theme
                enabled: !control.busy
                onClicked: control.editRequested()
            }

            PaletteButton {
                anchors.verticalCenter: parent.verticalCenter
                text: "Remove"
                tooltipText: "Remove this provider and its models"
                danger: true
                theme: control.theme
                enabled: !control.busy
                onClicked: control.removeRequested()
            }
        }

        Text {
            width: parent.width
            text: control.baseUrl
            color: control.c("text_muted", "#8a93a3")
            font.family: control.fontFamily
            font.pixelSize: control.fontSize("body_small", 11)
            elide: Text.ElideMiddle
        }

        Flow {
            width: parent.width
            spacing: 4
            visible: control.models.length > 0

            Repeater {
                model: control.models

                delegate: Rectangle {
                    height: 20
                    width: modelChipText.implicitWidth + 16
                    radius: control.cornerRadius(5)
                    color: control.c("surface_elevated", "#1b1f28")
                    border.width: 1
                    border.color: control.c("panel_border", "#2b303b")

                    Text {
                        id: modelChipText

                        anchors.centerIn: parent
                        text: String(modelData ?? "")
                        color: control.c("text", "#d3d8e2")
                        font.family: control.fontFamily
                        font.pixelSize: control.fontSize("body_small", 11)
                    }
                }
            }
        }
    }
}
