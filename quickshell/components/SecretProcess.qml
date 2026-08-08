import QtQuick
import Quickshell.Io

// One provider's API key: status, save, and delete. The key itself is written
// to harkctl's stdin and never appears in process arguments.
QtObject {
    id: secret

    required property var app
    required property string provider
    required property string label
    property string harkctlPath: "harkctl"

    property string keyInput: ""
    property bool configured: false
    property string source: "none"

    readonly property bool statusBusy: statusProcess.running
    readonly property bool saveBusy: setProcess.running
    readonly property bool deleteBusy: deleteProcess.running

    function loadStatus() {
        if (!statusProcess.running)
            statusProcess.exec([secret.harkctlPath, "secret", "status", "--json", secret.provider]);
    }

    function save() {
        if (secret.keyInput.trim().length === 0 || setProcess.running)
            return;

        secret.app.statusText = "Saving API key...";
        setProcess.exec([secret.harkctlPath, "secret", "set", "--stdin", secret.provider]);
    }

    function remove() {
        if (deleteProcess.running)
            return;

        secret.app.statusText = "Deleting API key...";
        deleteProcess.exec([secret.harkctlPath, "secret", "delete", secret.provider]);
    }

    property Process statusProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                if (line.length === 0)
                    return;

                try {
                    const status = JSON.parse(line);
                    secret.configured = Boolean(status.configured);
                    secret.source = String(status.source ?? "none");
                } catch (error) {
                    secret.app.statusText = "Secret status parse failed";
                }
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    secret.app.statusText = line;
            }
        }
    }

    property Process setProcess: Process {
        stdinEnabled: true
        onStarted: {
            write(secret.keyInput.trim() + "\n");
            stdinEnabled = false;
        }
        onExited: (exitCode) => {
            stdinEnabled = true;
            if (exitCode === 0) {
                secret.keyInput = "";
                secret.app.statusText = secret.label + " key saved";
                secret.loadStatus();
            } else if (secret.app.statusText.length === 0) {
                secret.app.statusText = "Secret save failed";
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    secret.app.statusText = line;
            }
        }
    }

    property Process deleteProcess: Process {
        onExited: (exitCode) => {
            if (exitCode === 0) {
                secret.keyInput = "";
                secret.app.statusText = secret.label + " key deleted";
                secret.loadStatus();
            } else if (secret.app.statusText.length === 0) {
                secret.app.statusText = "Secret delete failed";
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    secret.app.statusText = line;
            }
        }
    }
}
