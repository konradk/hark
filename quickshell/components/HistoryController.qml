import QtQuick
import Quickshell.Io

QtObject {
    id: root

    required property var app
    property string harkctlPath: "harkctl"

    property var historyModel: ListModel {
    }

    property int historySelectionIndex: -1
    property bool confirmClearHistory: false

    function reset() {
        historySelectionIndex = -1;
        historyModel.clear();
    }

    function loadHistory() {
        if (app.settings.showRecentChats && !historyProcess.running)
            historyProcess.exec([harkctlPath, "history", "list", "--limit", "10", "--json"]);

    }

    function handleHistory(line) {
        if (line.length === 0)
            return ;

        try {
            const entries = JSON.parse(line);
            historyModel.clear();
            for (const entry of entries) {
                historyModel.append({
                    "entryId": entry.id ?? 0,
                    "prompt": entry.prompt ?? "",
                    "response": entry.response ?? ""
                });
            }
            if (historySelectionIndex >= historyModel.count)
                historySelectionIndex = -1;

        } catch (error) {
            app.statusText = "History parse failed";
        }
    }

    function moveHistorySelection(delta) {
        if (!app.showHistorySuggestions || historyModel.count === 0)
            return false;

        if (historySelectionIndex < 0)
            historySelectionIndex = delta > 0 ? 0 : historyModel.count - 1;
        else
            historySelectionIndex = (historySelectionIndex + delta + historyModel.count) % historyModel.count;
        return true;
    }

    function openSelectedHistory() {
        if (!app.showHistorySuggestions || historySelectionIndex < 0 || historySelectionIndex >= historyModel.count)
            return false;

        const entry = historyModel.get(historySelectionIndex);
        openHistoryEntry(Number(entry.entryId ?? 0));
        return true;
    }

    function openHistoryEntry(entryId) {
        if (entryId <= 0 || historyGetProcess.running)
            return ;

        app.statusText = "Loading conversation...";
        historyGetProcess.exec([harkctlPath, "history", "get", "--json", String(entryId)]);
    }

    function handleHistoryEntry(line) {
        if (line.length === 0)
            return ;

        try {
            const entry = JSON.parse(line);
            const restoredMessages = [];
            if (entry.messages && Array.isArray(entry.messages)) {
                for (const message of entry.messages) {
                    const role = String(message.role ?? "");
                    const content = String(message.content ?? "").trim();
                    if ((role === "user" || role === "assistant") && content.length > 0)
                        restoredMessages.push({
                            "role": role,
                            "content": content
                        });

                }
            }
            if (restoredMessages.length === 0) {
                restoredMessages.push({
                    "role": "user",
                    "content": String(entry.prompt ?? "")
                }, {
                    "role": "assistant",
                    "content": String(entry.response ?? "")
                });
            }

            let latestAnswer = "";
            for (let index = restoredMessages.length - 1; index >= 0; index--) {
                if (restoredMessages[index].role === "assistant") {
                    latestAnswer = restoredMessages[index].content;
                    break;
                }
            }

            let attachmentPath = "";
            if (entry.attachments && entry.attachments.length > 0)
                attachmentPath = String(entry.attachments[0].path ?? "");

            historySelectionIndex = -1;
            app.statusText = "";
            app.applyRestoredConversation(String(entry.conversation_id ?? ""), restoredMessages, latestAnswer, attachmentPath);
        } catch (error) {
            app.statusText = "Could not load conversation";
        }
    }

    function deleteHistoryEntry(entryId) {
        if (entryId <= 0 || historyDeleteProcess.running)
            return ;

        app.statusText = "Deleting...";
        historyDeleteProcess.exec([harkctlPath, "history", "delete", String(entryId)]);
    }

    function clearAllHistory() {
        if (historyClearProcess.running)
            return ;

        if (!confirmClearHistory) {
            confirmClearHistory = true;
            clearHistoryConfirmTimer.restart();
            app.statusText = "Click “Clear all” again to confirm";
            return ;
        }

        clearHistoryConfirmTimer.stop();
        confirmClearHistory = false;
        app.statusText = "Clearing history...";
        historyClearProcess.exec([harkctlPath, "history", "clear", "--yes"]);
    }

    property Timer clearHistoryConfirmTimer: Timer {
        interval: 5000
        repeat: false
        onTriggered: root.confirmClearHistory = false
    }

    property Process historyProcess: Process {
        stdout: SplitParser {
            onRead: (line) => {
                return root.handleHistory(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

    property Process historyGetProcess: Process {
        onExited: (exitCode, exitStatus) => {
            if (exitCode !== 0) {
                if (root.app.statusText.length === 0 || root.app.statusText === "Loading conversation...")
                    root.app.statusText = "Could not load conversation";

            }
        }

        stdout: SplitParser {
            onRead: (line) => {
                return root.handleHistoryEntry(line);
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

    property Process historyDeleteProcess: Process {
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0) {
                if (root.app.statusText.length === 0 || root.app.statusText === "Deleting...")
                    root.app.statusText = "Deleted";
                root.loadHistory();
            } else if (root.app.statusText.length === 0) {
                root.app.statusText = "Delete failed";
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line;

            }
        }

    }

    property Process historyClearProcess: Process {
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0) {
                root.historyModel.clear();
                root.historySelectionIndex = -1;
                root.app.clearAttachment();
                if (root.app.statusText === "Clearing history...")
                    root.app.statusText = "History cleared";
            } else if (root.app.statusText.length === 0 || root.app.statusText === "Clearing history...") {
                root.app.statusText = "Could not clear history";
            }
        }

        stderr: SplitParser {
            onRead: (line) => {
                if (line.length > 0)
                    root.app.statusText = line.replace(/^harkctl:\s*/, "");

            }
        }

    }

}
