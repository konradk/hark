import QtQuick
import qs.Ui

BarWidget {
    id: root

    moduleName: "hark"

    // Hidden from the settings panel via `omarchy bar set hark hidden true`.
    // A zero-size, invisible widget leaves no gap in the bar.
    readonly property bool hidden: {
        const value = setting("hidden", false);
        return value === true || value === "true";
    }

    implicitWidth: hidden ? 0 : button.implicitWidth
    implicitHeight: hidden ? 0 : button.implicitHeight
    visible: !hidden

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
