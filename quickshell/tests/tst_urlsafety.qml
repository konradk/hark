import QtQuick
import QtTest
import "../js/UrlSafety.js" as UrlSafety

TestCase {
    name: "UrlSafety"

    function test_acceptsWebUrls_data() {
        return [
            { "tag": "https", "input": "https://example.com/path?q=one", "expected": "https://example.com/path?q=one" },
            { "tag": "http", "input": "http://localhost:8080/test", "expected": "http://localhost:8080/test" },
            { "tag": "trimmed", "input": "  https://example.com  ", "expected": "https://example.com" }
        ];
    }

    function test_acceptsWebUrls(data) {
        compare(UrlSafety.safeExternalUrl(data.input), data.expected);
    }

    function test_rejectsUnsafeUrls_data() {
        return [
            { "tag": "javascript", "input": "javascript:alert(1)" },
            { "tag": "file", "input": "file:///etc/passwd" },
            { "tag": "mailto", "input": "mailto:user@example.com" },
            { "tag": "custom scheme", "input": "custom://action" },
            { "tag": "missing host", "input": "https:///path" },
            { "tag": "embedded newline", "input": "https://example.com/\nfile:///tmp/x" },
            { "tag": "backslash", "input": "https://example.com\\@evil.test" },
            { "tag": "empty", "input": "" }
        ];
    }

    function test_rejectsUnsafeUrls(data) {
        compare(UrlSafety.safeExternalUrl(data.input), "");
    }

    function test_escapesHtmlAttributes() {
        compare(
            UrlSafety.escapeHtmlAttribute("https://example.com/?a=\"x\"&b='<tag>'"),
            "https://example.com/?a=&quot;x&quot;&amp;b=&#39;&lt;tag&gt;&#39;"
        );
    }

    function test_roundTripsMarkdownEscapingOnce() {
        const escapedMarkdownUrl = "https://example.com/?a=1&amp;b=2";
        const decoded = UrlSafety.decodeHtmlAttribute(escapedMarkdownUrl);
        compare(decoded, "https://example.com/?a=1&b=2");
        compare(
            UrlSafety.escapeHtmlAttribute(UrlSafety.safeExternalUrl(decoded)),
            "https://example.com/?a=1&amp;b=2"
        );
    }
}
