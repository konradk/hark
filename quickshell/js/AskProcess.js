.pragma library

function shouldHandleErrorLine(line, stopRequested) {
    return String(line).length > 0 && !stopRequested;
}

function shouldReportExit(exitCode, stopRequested, hasError) {
    return Number(exitCode) !== 0 && !stopRequested && !hasError;
}
