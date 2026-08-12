import QtQuick
import QtTest
import "../js/StreamReveal.js" as StreamReveal

TestCase {
    name: "StreamReveal"

    function test_emptyBufferRevealsNothing() {
        compare(StreamReveal.unitsForTick(0, 32, 640, 24), 0);
    }

    function test_smallBufferSpreadsAcrossTargetTime() {
        compare(StreamReveal.unitsForTick(100, 32, 640, 24), 5);
    }

    function test_liveBacklogHasPerFrameLimit() {
        compare(StreamReveal.unitsForTick(2000, 32, 640, 24), 24);
    }

    function test_completedStreamCanCatchUpWithoutLimit() {
        compare(StreamReveal.unitsForTick(2000, 32, 1600, 2000), 40);
    }
}
