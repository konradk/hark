import QtQuick
import qs.Ui

BarWidget {
    id: root

    moduleName: "hark"
    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    BarIconButton {
        id: button

        anchors.fill: parent
        bar: root.bar
        text: "󰙴"
        tooltipText: "Open Hark"
        onPressed: function() {
            if (root.bar)
                root.bar.run("omarchy-shell shell toggle hark '{}'");
        }
    }
}
