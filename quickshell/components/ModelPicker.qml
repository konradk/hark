import QtQuick

PaletteComboBox {
    id: picker

    property string valueField: "modelId"

    width: 132
    height: 28
    textRole: "label"
    valueRole: valueField
}
