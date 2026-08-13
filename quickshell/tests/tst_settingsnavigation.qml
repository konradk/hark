import QtQuick
import QtTest
import "../js/SettingsNavigation.js" as SettingsNavigation

TestCase {
    name: "SettingsNavigation"

    function test_opensFromMainView() {
        compare(SettingsNavigation.toggleAction(false, false, false), "open");
    }

    function test_returnsToMainBeforeOpeningFromConversation() {
        compare(SettingsNavigation.toggleAction(false, true, false), "open-main");
    }

    function test_doesNotLeaveAnActiveResponse() {
        compare(SettingsNavigation.toggleAction(false, true, true), "blocked");
    }

    function test_closesRegardlessOfConversationState() {
        compare(SettingsNavigation.toggleAction(true, true, true), "close");
    }
}
