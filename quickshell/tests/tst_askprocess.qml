import QtQuick
import QtTest
import "../js/AskProcess.js" as AskProcess

TestCase {
    name: "AskProcess"

    function test_intentionalStopSuppressesExitAndStderr() {
        verify(!AskProcess.shouldReportExit(15, true, false));
        verify(!AskProcess.shouldHandleErrorLine("terminated", true));
    }

    function test_unexpectedFailureIsReportedOnce() {
        verify(AskProcess.shouldReportExit(15, false, false));
        verify(!AskProcess.shouldReportExit(15, false, true));
        verify(AskProcess.shouldHandleErrorLine("provider failed", false));
    }

    function test_successIsNotReported() {
        verify(!AskProcess.shouldReportExit(0, false, false));
    }
}
