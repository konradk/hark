import QtQuick
import Quickshell
import Quickshell.Io
import "plugin" as HarkPlugin
import "." as HarkRoot

QtObject {
    id: root

    readonly property string repoDir: Quickshell.env("HARK_TEST_REPO")
    property var testManifest: ({
        "id": "hark",
        "__sourceDir": root.repoDir
    })

    property HarkPlugin.Service service: HarkPlugin.Service {
        manifest: root.testManifest
    }

    property QtObject testShell: QtObject {
        function serviceFor(pluginId) {
            return pluginId === "hark" ? root.service : null;
        }
    }

    property HarkRoot.Overlay overlay: HarkRoot.Overlay {
        manifest: root.testManifest
        shell: root.testShell
    }

    property IpcHandler ipc: IpcHandler {
        target: "hark-plugin-test"

        function status(): string {
            return JSON.stringify({
                "phase": root.service.phase,
                "ready": root.service.ready,
                "ownsDaemon": root.service.ownsDaemon,
                "overlayBackendReady": root.overlay.backendReady,
                "usesBundledHarkctl": root.overlay.overlay.harkctlPath === root.repoDir + "/bin/harkctl",
                "error": root.service.lastError
            });
        }
    }
}
