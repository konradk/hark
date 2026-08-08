import QtQuick
import QtTest
import "../js/Markdown.js" as Markdown

TestCase {
    name: "Markdown"

    readonly property var theme: ({
        "text_strong": "#f0f0f0",
        "text_muted": "#888888",
        "text_disabled": "#555555"
    })

    function test_normalizesLegacySources() {
        const input = "Answer [source 2](https://example.com/two)\n\n"
            + "## Sources\n\n"
            + "1. [One](https://example.com/one)\n"
            + "2. [Two](https://example.com/two)\n";
        compare(
            Markdown.normalizeLegacySources(input),
            "Answer [[2]](https://example.com/two)\n\nSources: [[2]](https://example.com/two) Two"
        );
    }

    function test_rendersSafeLinksAndRejectsUnsafeLinks() {
        const safe = Markdown.inlineMarkdownToHtml("[Example](https://example.com/?a=1&b=2)", theme);
        verify(safe.indexOf("href=\"https://example.com/?a=1&amp;b=2\"") >= 0);

        const unsafe = Markdown.inlineMarkdownToHtml("[Open](javascript:alert(1))", theme);
        verify(unsafe.indexOf("href=") < 0);
        verify(unsafe.indexOf("Open") >= 0);
    }

    function test_parsesStructuredContent() {
        const segments = Markdown.parseContent(
            "# Heading\n\n"
            + "```go\nfmt.Println(\"ok\")\n```\n\n"
            + "| Name | Value |\n| --- | --- |\n| one | two |\n\n"
            + "Sources: [[1]](https://example.com) Example",
            true,
            false
        );

        compare(segments.length, 4);
        compare(segments[0].kind, "heading");
        compare(segments[1].kind, "code");
        compare(segments[1].language, "go");
        compare(segments[2].kind, "table");
        compare(segments[2].columns, 2);
        compare(segments[3].kind, "sources");
        compare(segments[3].items[0].title, "Example");
    }

    function test_keepsUserContentAsPlainMarkdownSegment() {
        const segments = Markdown.parseContent("# not a heading", false, false);
        compare(segments.length, 1);
        compare(segments[0].kind, "markdown");
        compare(segments[0].text, "# not a heading");
    }

    function test_rendersLists() {
        const html = Markdown.markdownBlockToHtml("- one\n- **two**", theme);
        verify(html.indexOf("<ul>") >= 0);
        verify(html.indexOf("<li>one</li>") >= 0);
        verify(html.indexOf("<li><b>two</b></li>") >= 0);
    }
}
