import QtQuick
import Quickshell
import Quickshell.Io

// Stops only the daemon it starts.
QtObject {
    id: root

    // Set by Omarchy.
    property var shell: null
    property var manifest: null

    readonly property string pluginDir: manifest && manifest.__sourceDir ? String(manifest.__sourceDir) : ""
    readonly property string harkctlPath: pluginDir.length > 0 ? pluginDir + "/bin/harkctl" : "harkctl"
    readonly property string harkdPath: pluginDir.length > 0 ? pluginDir + "/bin/harkd" : "harkd"

    property string phase: "idle"
    property string lastError: ""
    property bool ready: false
    property bool ownsDaemon: false
    property bool stopping: false
    property bool waitingForOwnedDaemon: false
    property int healthAttempts: 0
    property int restartAttempts: 0

    readonly property int maxHealthAttempts: 50
    readonly property int healthIntervalMs: 100
    readonly property int maxRestartDelayMs: 30000
    readonly property int protocolVersion: 3
    readonly property int incompatibleProtocolExitCode: 3

    function reconcile() {
        if (root.stopping || root.pluginDir.length === 0 || statusProcess.running)
            return;

        root.phase = root.waitingForOwnedDaemon ? "starting" : "checking";
        statusProcess.command = [root.harkctlPath, "-timeout", "2s", "status", "--json", "--require-protocol", String(root.protocolVersion)];
        statusProcess.running = true;
    }

    function startDaemon() {
        if (root.stopping || daemonProcess.running)
            return;

        root.ready = false;
        root.phase = "starting";
        root.lastError = "";
        root.waitingForOwnedDaemon = true;
        root.healthAttempts = 0;
        daemonProcess.command = [root.harkdPath];
        daemonProcess.running = true;
    }

    function markReady(owned) {
        root.ready = true;
        root.phase = "ready";
        root.lastError = "";
        root.waitingForOwnedDaemon = false;
        root.healthAttempts = 0;
        root.restartAttempts = 0;
        root.ownsDaemon = owned;
        healthTimer.stop();
        restartTimer.stop();
    }

    function markIncompatible(message) {
        root.ready = false;
        root.phase = "incompatible";
        root.lastError = message.length > 0 ? message : "An incompatible Hark daemon is already running. Stop or upgrade it, then reload the plugin.";
        root.waitingForOwnedDaemon = false;
        root.healthAttempts = 0;
        healthTimer.stop();
        restartTimer.stop();
    }

    function scheduleHealthCheck() {
        if (root.stopping)
            return;

        root.healthAttempts += 1;
        if (root.healthAttempts >= root.maxHealthAttempts) {
            root.phase = "error";
            root.lastError = "Hark daemon did not become ready in time.";
            if (root.ownsDaemon && daemonProcess.running)
                daemonProcess.running = false;
            return;
        }
        healthTimer.restart();
    }

    function scheduleRestart(message) {
        if (root.stopping)
            return;

        root.ready = false;
        root.ownsDaemon = false;
        root.waitingForOwnedDaemon = false;
        root.restartAttempts += 1;
        root.phase = "restarting";
        root.lastError = message;
        restartTimer.interval = Math.min(root.maxRestartDelayMs, 500 * Math.pow(2, Math.min(root.restartAttempts - 1, 6)));
        restartTimer.restart();
    }

    onManifestChanged: Qt.callLater(root.reconcile)

    property Process statusProcess: Process {
        stdout: StdioCollector {
            waitForEnd: true
        }

        stderr: StdioCollector {
            id: statusError
            waitForEnd: true
        }

        onExited: function(exitCode) {
            if (root.stopping)
                return;

            if (exitCode === 0) {
                root.markReady(root.waitingForOwnedDaemon && daemonProcess.running);
                return;
            }

            if (exitCode === root.incompatibleProtocolExitCode) {
                const message = String(statusError.text ?? "").replace(/^harkctl:\s*/, "").trim();
                root.markIncompatible(message);
                return;
            }

            if (root.waitingForOwnedDaemon && daemonProcess.running) {
                root.scheduleHealthCheck();
                return;
            }

            root.startDaemon();
        }
    }

    property Process daemonProcess: Process {
        stderr: SplitParser {
            onRead: function(line) {
                // Never expose daemon stderr through plugin IPC.
                if (line.length > 0)
                    console.warn("harkd: " + line);
            }
        }

        onStarted: {
            root.ownsDaemon = true;
            root.waitingForOwnedDaemon = true;
            root.scheduleHealthCheck();
        }

        onExited: function(exitCode) {
            var wasOwned = root.ownsDaemon;
            root.ownsDaemon = false;
            root.ready = false;
            if (root.stopping)
                return;

            // Recheck after losing a socket race.
            root.waitingForOwnedDaemon = false;
            root.scheduleRestart(wasOwned ? "Hark daemon exited with code " + exitCode + "." : "Could not start the bundled Hark daemon.");
        }
    }

    property Timer healthTimer: Timer {
        interval: root.healthIntervalMs
        repeat: false
        onTriggered: root.reconcile()
    }

    property Timer restartTimer: Timer {
        interval: 500
        repeat: false
        onTriggered: root.reconcile()
    }

    Component.onCompleted: Qt.callLater(root.reconcile)
    Component.onDestruction: {
        root.stopping = true;
        healthTimer.stop();
        restartTimer.stop();
        statusProcess.running = false;
        if (root.ownsDaemon && daemonProcess.running)
            daemonProcess.running = false;
    }
}
