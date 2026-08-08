import QtQuick
import QtTest
import "../components"

TestCase {
    id: testCase

    name: "SettingsPanel"

    ListModel {
        id: retentionOptions

        ListElement { label: "Forever"; days: 0 }
        ListElement { label: "30 days"; days: 30 }
    }

    Component {
        id: panelComponent

        SettingsPanel {
            width: 720
            retentionModel: retentionOptions
        }
    }

    function test_createsWithCurrentSettings() {
        const panel = createTemporaryObject(panelComponent, testCase, {
            "showRecentChats": false,
            "saveHistory": true,
            "historyRetentionDays": 30,
            "globalShortcut": "SUPER + A",
            "globalShortcutConfigured": true
        });
        verify(panel !== null);
        compare(panel.showRecentChats, false);
        compare(panel.saveHistory, true);
        compare(panel.globalShortcut, "SUPER + A");
        compare(panel.height, panel.implicitHeight);
        verify(panel.implicitHeight > 0);
    }

    function test_addsCustomRetentionValueOnce() {
        const panel = createTemporaryObject(panelComponent, testCase, {
            "historyRetentionDays": 45
        });
        verify(panel !== null);
        panel.syncRetention();
        compare(retentionOptions.count, 3);
        panel.syncRetention();
        compare(retentionOptions.count, 3);
        retentionOptions.remove(2);
    }
}
