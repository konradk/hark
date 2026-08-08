import QtQuick
import Quickshell.Io

// One daemon-stored setting: a `harkctl setting get` that parses the stored
// value and a `harkctl setting set` that reports failures and rolls the
// optimistic UI update back by reloading.
QtObject {
    id: setting

    required property var app
    required property string key
    property string harkctlPath: "harkctl"
    property string savingText: ""
    property string failureText: "Could not save setting"

    readonly property bool busy: setProcess.running

    signal loaded(var value, bool found)
    signal saved()

    function load() {
        if (!getProcess.running)
            getProcess.exec([setting.harkctlPath, "setting", "get", setting.key]);
    }

    function save(value) {
        if (setProcess.running)
            return false;

        if (setting.savingText.length > 0)
            setting.app.statusText = setting.savingText;

        setProcess.exec([setting.harkctlPath, "setting", "set", setting.key, String(value)]);
        return true;
    }

    // Only overwrite the status line if it still shows this save in progress,
    // so a newer message from elsewhere is not clobbered.
    function reportSaved(text) {
        if (setting.savingText.length === 0 || setting.app.statusText === setting.savingText)
            setting.app.statusText = text;
    }

    property Process getProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                if (line.length === 0)
                    return;

                try {
                    const parsed = JSON.parse(line);
                    setting.loaded(parsed.value, Boolean(parsed.found));
                } catch (error) {
                    setting.app.statusText = setting.key + " setting parse failed";
                }
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    setting.app.statusText = line.replace(/^harkctl:\s*/, "");
            }
        }
    }

    property Process setProcess: Process {
        onExited: (exitCode) => {
            if (exitCode === 0) {
                setting.saved();
                return;
            }
            setting.app.statusText = setting.failureText;
            setting.load();
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    setting.app.statusText = line.replace(/^harkctl:\s*/, "");
            }
        }
    }
}
