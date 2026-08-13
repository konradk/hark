.pragma library

function toggleAction(settingsExpanded, hasThread, responseBusy) {
    if (settingsExpanded)
        return "close";
    if (responseBusy)
        return "blocked";
    if (hasThread)
        return "open-main";
    return "open";
}
