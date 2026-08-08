import QtQuick
import "quickshell" as Hark

Item {
    id: root

    // Set by Omarchy.
    property var shell: null
    property var manifest: null
    property var service: null

    property bool animateNextClose: true
    readonly property var effectiveService: root.service ? root.service : root.shell && typeof root.shell.serviceFor === "function" ? root.shell.serviceFor(root.pluginId()) : null
    readonly property bool opened: overlay.visible
    readonly property bool backendReady: overlay.backendReady

    function pluginId() {
        return root.manifest && root.manifest.id ? String(root.manifest.id) : "hark";
    }

    function syncService() {
        const currentService = root.effectiveService;
        const pluginDir = root.manifest && root.manifest.__sourceDir ? String(root.manifest.__sourceDir) : "";
        if (currentService && currentService.harkctlPath)
            overlay.harkctlPath = String(currentService.harkctlPath);
        else if (pluginDir.length > 0)
            overlay.harkctlPath = pluginDir + "/bin/harkctl";

        if (currentService && !currentService.ready && currentService.lastError)
            overlay.statusText = String(currentService.lastError);
        overlay.backendReady = currentService ? currentService.ready : false;
    }

    function open(payloadJson) {
        root.syncService();
        overlay.openFromHost(payloadJson || "");
    }

    function close() {
        overlay.closeFromHost(root.animateNextClose);
        root.animateNextClose = true;
    }

    function requestOpen(payloadJson) {
        if (root.shell && typeof root.shell.summon === "function")
            root.shell.summon(root.pluginId(), payloadJson || "");
        else
            root.open(payloadJson || "");
    }

    function requestClose(animated) {
        root.animateNextClose = animated !== false;
        if (root.shell && typeof root.shell.hide === "function")
            root.shell.hide(root.pluginId());
        else
            root.close();
    }

    onServiceChanged: root.syncService()
    onEffectiveServiceChanged: root.syncService()
    onManifestChanged: root.syncService()

    Connections {
        target: root.effectiveService

        function onReadyChanged() {
            root.syncService();
        }

        function onLastErrorChanged() {
            root.syncService();
        }
    }

    property Hark.HarkShell overlay: Hark.HarkShell {
        hostMode: "omarchy"
        backendReady: false
        Component.onCompleted: root.syncService()
    }

    Connections {
        target: overlay

        function onOpenRequested(payloadJson) {
            root.requestOpen(payloadJson);
        }

        function onCloseRequested(animated) {
            root.requestClose(animated);
        }
    }
}
