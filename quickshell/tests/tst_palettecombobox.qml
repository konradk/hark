import QtQuick
import QtTest
import "../components"

TestCase {
    id: testCase

    name: "PaletteComboBox"
    when: windowShown

    ListModel {
        id: options

        ListElement { label: "Short"; modelId: "short" }
        ListElement { label: "Claude Opus 5 (OpenRouter)"; modelId: "anthropic/claude-opus-5" }
    }

    Component {
        id: pickerComponent

        ModelPicker {
            width: 128
            model: options
        }
    }

    function test_popupFitsLongLabelsAndAlignsToRightEdge() {
        const picker = createTemporaryObject(pickerComponent, testCase, {
            "x": 200
        });
        verify(picker !== null);
        picker.popup.open();
        wait(0);
        verify(picker.popup.width > picker.width);
        verify(picker.popup.width <= picker.popupMaximumWidth);
        compare(picker.popup.x + picker.popup.width, picker.width);
        picker.popup.close();
    }

    function test_popupWidthIsBounded() {
        const picker = createTemporaryObject(pickerComponent, testCase, {
            "popupMaximumWidth": 180
        });
        verify(picker !== null);
        compare(picker.popup.width, 180);
    }
}
