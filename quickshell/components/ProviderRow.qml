import QtQuick

Item {
    id: control

    property string providerId: ""
    property string label: ""
    property string baseUrl: ""
    property var models: []
    property bool busy: false
    property bool fetched: false
    property var fetchedModels: []
    property bool fetchBusy: false
    property var theme: null
    property string fontFamily: ""

    signal editRequested()
    signal removeRequested()
    signal modelAddRequested(string modelId)
    signal modelRemoveRequested(string modelId)
    signal fetchRequested()

    property string modelInput: ""

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

    function resetModelInput() {
        modelInput = "";
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
                    height: 22
                    width: modelRow.implicitWidth + 20
                    radius: control.cornerRadius(5)
                    color: control.c("surface_elevated", "#1b1f28")
                    border.width: 1
                    border.color: control.c("panel_border", "#2b303b")

                    Row {
                        id: modelRow

                        anchors.verticalCenter: parent.verticalCenter
                        anchors.left: parent.left
                        anchors.leftMargin: 8
                        spacing: 6

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: String(modelData.id ?? "")
                            color: control.c("text", "#d3d8e2")
                            font.family: control.fontFamily
                            font.pixelSize: control.fontSize("body_small", 11)
                        }

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "\u00d7"
                            color: control.c("text_muted", "#8a93a3")
                            font.family: control.fontFamily
                            font.pixelSize: control.fontSize("body_small", 12)

                            MouseArea {
                                anchors.fill: parent
                                anchors.margins: -6
                                cursorShape: Qt.PointingHandCursor
                                onClicked: control.modelRemoveRequested(String(modelData.id ?? ""))
                            }
                        }
                    }
                }
            }
        }

        Row {
            width: parent.width
            spacing: 6

            PaletteButton {
                anchors.verticalCenter: parent.verticalCenter
                text: control.fetchBusy ? "Fetching" : "Fetch"
                tooltipText: "List the models this endpoint exposes"
                theme: control.theme
                enabled: !control.fetchBusy && !control.busy
                onClicked: control.fetchRequested()
            }

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(80, parent.width - 2 * 84 - 24)
                height: 26
                radius: control.cornerRadius(6)
                color: control.c("input", "#0d1016")
                border.width: 1
                border.color: modelIdField.activeFocus ? control.c("primary", "#a7c7ff") : control.c("panel_border", "#2b303b")

                TextInput {
                    id: modelIdField

                    anchors.fill: parent
                    anchors.leftMargin: 8
                    anchors.rightMargin: 8
                    text: control.modelInput
                    color: control.c("text_strong", "#f2f4f8")
                    font.family: control.fontFamily
                    font.pixelSize: control.fontSize("body_small", 11)
                    verticalAlignment: TextInput.AlignVCenter
                    clip: true
                    onTextChanged: control.modelInput = text
                }
            }

            PaletteButton {
                anchors.verticalCenter: parent.verticalCenter
                text: "Add"
                tooltipText: "Add this model to the provider"
                theme: control.theme
                filled: true
                primary: control.modelInput.trim().length > 0
                enabled: control.modelInput.trim().length > 0 && !control.busy
                onClicked: {
                    control.modelAddRequested(control.modelInput.trim());
                    control.resetModelInput();
                }
            }
        }

        Flow {
            width: parent.width
            spacing: 4
            visible: control.fetched && control.fetchedModels.length > 0

            Repeater {
                model: control.fetchedModels

                delegate: Rectangle {
                    height: 22
                    width: fetchedLabel.implicitWidth + 18
                    radius: control.cornerRadius(5)
                    color: control.c("surface_hover", "#222733")
                    border.width: 1
                    border.color: control.c("panel_border", "#2b303b")

                    Text {
                        id: fetchedLabel

                        anchors.centerIn: parent
                        text: String(modelData)
                        color: control.c("text", "#d3d8e2")
                        font.family: control.fontFamily
                        font.pixelSize: control.fontSize("body_small", 11)
                    }

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: control.modelAddRequested(String(modelData))
                    }
                }
            }
        }
    }
}
