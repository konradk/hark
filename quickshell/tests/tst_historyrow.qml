import QtQuick
import QtTest
import "../components"

TestCase {
    id: testCase

    name: "HistoryRow"

    Component {
        id: rowComponent

        HistoryRow {
            width: 640
        }
    }

    function test_multilinePromptStaysOnOneRow() {
        const row = createTemporaryObject(rowComponent, testCase, {
            "promptText": "Proposed change\n1. First option\n2. Second option"
        });
        verify(row !== null);

        const promptPreview = findChild(row, "historyPromptPreview");
        verify(promptPreview !== null);
        compare(promptPreview.text, "Proposed change 1. First option 2. Second option");
        compare(promptPreview.maximumLineCount, 1);
        compare(promptPreview.lineCount, 1);
    }
}
