.pragma library
.import "UrlSafety.js" as UrlSafety

function color(theme, name, fallback) {
    return theme && theme[name] ? theme[name] : fallback;
}

function normalizeLegacySources(value) {
    let text = String(value ?? "");
    const citedIndexes = {};
    const legacyCitationPattern = /\[source\s+(\d+)\]\(([^)\s]+)\)/gi;
    let citationMatch;
    while ((citationMatch = legacyCitationPattern.exec(text)) !== null)
        citedIndexes[citationMatch[1]] = true;

    text = text.replace(/\[source\s+(\d+)\]\(([^)\s]+)\)/gi, "[[$1]]($2)");
    const marker = text.match(/\n+#{1,6}\s+Sources\s*\n+/i);
    if (!marker)
        return text;

    const before = text.slice(0, marker.index).replace(/\s+$/, "");
    const lines = text.slice(marker.index + marker[0].length).split("\n");
    const compact = [];
    let consumed = 0;
    for (const line of lines) {
        const source = line.match(/^\s*(\d+)\.\s+\[([^\]]+)\]\(([^)\s]+)\)\s*$/);
        if (!source)
            break;

        if (Object.keys(citedIndexes).length === 0 || citedIndexes[source[1]])
            compact.push("[[" + source[1] + "]](" + source[3] + ") " + source[2]);
        consumed++;
    }
    if (compact.length === 0)
        return text;

    const rest = lines.slice(consumed).join("\n").replace(/^\s+/, "");
    return before + "\n\nSources: " + compact.join(" · ") + (rest.length > 0 ? "\n\n" + rest : "");
}

function escapeHtml(value) {
    return String(value ?? "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function anchorHtml(label, href, linkColor) {
    const safeHref = UrlSafety.safeExternalUrl(UrlSafety.decodeHtmlAttribute(href));
    if (safeHref.length === 0)
        return label;

    return "<a href=\"" + UrlSafety.escapeHtmlAttribute(safeHref) + "\" style=\"color:"
        + UrlSafety.escapeHtmlAttribute(linkColor) + "; text-decoration:underline\">" + label + "</a>";
}

function sourceItemsFromLine(value) {
    const text = String(value ?? "").replace(/^\s*Sources:\s*/i, "");
    const pattern = /\[\[(\d+)\]\]\(([^)\s]+)\)\s+(.+?)(?=\s+·\s+\[\[\d+\]\]\(|$)/g;
    const items = [];
    let match;
    while ((match = pattern.exec(text)) !== null) {
        items.push({
            "index": match[1],
            "url": match[2],
            "title": match[3].trim()
        });
    }
    return items;
}

function sourcesHtml(items, theme) {
    const textColor = color(theme, "text_muted", "#8a93a3");
    const separator = "<span style=\"color:" + color(theme, "text_disabled", "#5b6373") + "\">  ·  </span>";
    return items.map(function(source) {
        const label = "[" + source.index + "] " + source.title;
        return anchorHtml(escapeHtml(label), source.url, textColor);
    }).join(separator);
}

function inlineMarkdownToHtml(value, theme) {
    let text = escapeHtml(value);
    const codeSpans = [];
    text = text.replace(/`([^`]+)`/g, function(match, codeText) {
        codeSpans.push(codeText);
        return "\u0000" + (codeSpans.length - 1) + "\u0000";
    });
    text = text.replace(/\[\[(\d+)\]\]\(([^)\s]+)\)/g, function(match, index, href) {
        return anchorHtml("[" + index + "]", href, color(theme, "text_muted", "#8a93a3"));
    });
    text = text.replace(/\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, function(match, label, href) {
        return anchorHtml(label, href, color(theme, "text_strong", "#f2f4f8"));
    });
    text = text.replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>");
    text = text.replace(/__([^_]+)__/g, "<b>$1</b>");
    text = text.replace(/\*([^*\n]+)\*/g, "<i>$1</i>");
    text = text.replace(/(^|[\s(])_([^_]+)_(?=$|[\s).,;:!?])/g, "$1<i>$2</i>");
    text = text.replace(/~~([^~]+)~~/g, "<s>$1</s>");
    text = text.replace(/\u0000(\d+)\u0000/g, function(match, index) {
        return "<code>" + codeSpans[Number(index)] + "</code>";
    });
    return text;
}

function markdownBlockToHtml(value, theme) {
    const lines = String(value ?? "").split("\n");
    let html = "";
    const stack = [];
    let paragraph = [];
    function flushParagraph() {
        if (paragraph.length === 0)
            return;

        const content = inlineMarkdownToHtml(paragraph.join(" "), theme);
        html += html.length > 0 ? "<p style=\"margin:0\">" + content + "</p>" : content;
        paragraph = [];
    }
    function closeLists(level) {
        while (stack.length > level)
            html += "</" + stack.pop() + ">";
    }
    for (const line of lines) {
        const item = line.match(/^(\s*)([-*+]|\d+[.)])\s+(.*)$/);
        if (item) {
            flushParagraph();
            const level = Math.floor(item[1].replace(/\t/g, "  ").length / 2) + 1;
            const type = /^\d/.test(item[2]) ? "ol" : "ul";
            closeLists(level);
            if (stack.length === level && stack[stack.length - 1] !== type)
                html += "</" + stack.pop() + ">";
            while (stack.length < level) {
                html += "<" + type + ">";
                stack.push(type);
            }
            html += "<li>" + inlineMarkdownToHtml(item[3], theme) + "</li>";
        } else if (line.trim().length === 0) {
            flushParagraph();
            closeLists(0);
        } else {
            closeLists(0);
            paragraph.push(line.trim());
        }
    }
    flushParagraph();
    closeLists(0);
    return html;
}

function parseTable(rawLines) {
    const rows = [];
    for (const raw of rawLines) {
        const trimmed = raw.trim().replace(/^\|/, "").replace(/\|$/, "");
        rows.push(trimmed.split("|").map(function(cell) {
            return cell.trim();
        }));
    }
    const dataRows = rows.filter(function(row) {
        return !(row.length > 0 && row.every(function(cell) {
            return /^:?-+:?$/.test(cell);
        }));
    });
    const hasSeparator = dataRows.length < rows.length;
    if (dataRows.length === 0)
        return null;

    const columns = Math.max(...dataRows.map(function(row) {
        return row.length;
    }));
    if (columns < 2)
        return null;

    const items = [];
    dataRows.forEach(function(row, rowIndex) {
        const headerRow = hasSeparator && rowIndex === 0;
        if (rowIndex === 1 && hasSeparator)
            items.push({
                "cellKind": "sep",
                "text": "",
                "header": false
            });

        for (let column = 0; column < columns; column++) {
            items.push({
                "cellKind": "cell",
                "text": String(row[column] ?? ""),
                "header": headerRow
            });
        }
    });
    return {
        "kind": "table",
        "columns": columns,
        "cells": items
    };
}

function parseContent(value, assistant, errorRole) {
    const text = (assistant ? normalizeLegacySources(value) : String(value ?? "")).replace(/\r\n/g, "\n");
    if (!assistant && !errorRole)
        return [{"kind": "markdown", "text": text}];

    const segments = [];
    const lines = text.split("\n");
    let buffer = [];
    let bufferKind = "";
    let code = [];
    let inCode = false;
    let language = "";
    function flush() {
        if (buffer.length === 0)
            return;

        if (bufferKind === "quote") {
            const quote = buffer.map(function(line) {
                return line.replace(/^\s*>\s?/, "");
            }).join("\n").trim();
            if (quote.length > 0)
                segments.push({"kind": "quote", "text": quote});
        } else if (bufferKind === "table") {
            const table = parseTable(buffer);
            if (table)
                segments.push(table);
            else
                segments.push({"kind": "markdown", "text": buffer.join("\n").trim()});
        } else {
            const chunk = buffer.join("\n").trim();
            if (chunk.length > 0)
                segments.push({"kind": "markdown", "text": chunk});
        }
        buffer = [];
        bufferKind = "";
    }

    for (const line of lines) {
        if (line.indexOf("```") === 0) {
            if (inCode) {
                segments.push({
                    "kind": "code",
                    "language": language,
                    "text": code.join("\n").replace(/\s+$/, "")
                });
                code = [];
                language = "";
                inCode = false;
            } else {
                flush();
                inCode = true;
                language = line.slice(3).trim();
            }
            continue;
        }
        if (inCode) {
            code.push(line);
            continue;
        }
        if (assistant && /^\s*Sources:\s*/i.test(line)) {
            const sourceItems = sourceItemsFromLine(line);
            if (sourceItems.length > 0) {
                flush();
                segments.push({"kind": "sources", "items": sourceItems});
                continue;
            }
        }
        const heading = line.match(/^(#{1,6})\s+(.*)$/);
        if (heading) {
            flush();
            segments.push({
                "kind": "heading",
                "level": heading[1].length,
                "text": heading[2].replace(/\s*#+\s*$/, "").trim()
            });
            continue;
        }
        if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
            flush();
            segments.push({"kind": "hr"});
            continue;
        }
        if (/^\s*>/.test(line)) {
            if (bufferKind !== "quote")
                flush();
            bufferKind = "quote";
            buffer.push(line);
            continue;
        }
        if (/^\s*\|/.test(line)) {
            if (bufferKind !== "table")
                flush();
            bufferKind = "table";
            buffer.push(line);
            continue;
        }
        if (line.trim().length === 0) {
            flush();
            continue;
        }
        if (bufferKind === "quote" || bufferKind === "table")
            flush();

        bufferKind = "markdown";
        buffer.push(line);
    }
    if (inCode) {
        segments.push({
            "kind": "code",
            "language": language,
            "text": code.join("\n").replace(/\s+$/, "")
        });
    } else {
        flush();
    }
    if (segments.length === 0)
        segments.push({"kind": "markdown", "text": text});

    return segments;
}
