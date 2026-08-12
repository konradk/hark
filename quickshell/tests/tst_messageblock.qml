import QtQuick
import QtTest
import "../components"

TestCase {
    id: testCase

    name: "MessageBlock"

    Component {
        id: blockComponent

        MessageBlock {
            width: 640
            pending: true
        }
    }

    function segmentText(block) {
        return block.contentSegments.length > 0 ? block.contentSegments[0].text : "";
    }

    function test_pendingContentRefreshesDuringContinuousStreaming() {
        const block = createTemporaryObject(blockComponent, testCase, {
            "content": "initial"
        });
        verify(block !== null);
        tryVerify(function() {
            return segmentText(block) === "initial";
        });

        block.content = "first delta";
        wait(30);
        block.content = "second delta";
        wait(30);

        compare(segmentText(block), "second delta");
    }

    function test_pendingActivityReplacesAnswerHeader() {
        const block = createTemporaryObject(blockComponent, testCase, {
            "activityText": "Searching the web..."
        });
        verify(block !== null);
        compare(block.headerText, "SEARCHING THE WEB");

        block.content = "First answer text";
        compare(block.headerText, "ANSWER");
    }
}
