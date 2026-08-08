.pragma library

function safeExternalUrl(value) {
    const url = String(value ?? "").trim();
    if (url.length === 0 || /[\u0000-\u001f\u007f\s\\]/.test(url))
        return "";

    const match = url.match(/^https?:\/\/([^/?#]+)(?:[/?#].*)?$/i);
    if (!match || match[1].length === 0)
        return "";

    return url;
}

function escapeHtmlAttribute(value) {
    return String(value ?? "")
        .replace(/&/g, "&amp;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");
}

function decodeHtmlAttribute(value) {
    return String(value ?? "")
        .replace(/&quot;/g, "\"")
        .replace(/&#39;/g, "'")
        .replace(/&lt;/g, "<")
        .replace(/&gt;/g, ">")
        .replace(/&amp;/g, "&");
}
