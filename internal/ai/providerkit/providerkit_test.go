package providerkit

import "testing"

func TestFormatCitedAnswerDropsSourcesWithInvalidRanges(t *testing.T) {
	answer, sources := FormatCitedAnswer("short answer", []Citation{{
		Title:      "Invalid",
		URL:        "https://example.com/invalid",
		StartIndex: 100,
		EndIndex:   110,
	}}, nil)
	if answer != "short answer" {
		t.Fatalf("answer = %q", answer)
	}
	if len(sources) != 0 {
		t.Fatalf("sources = %#v, want none", sources)
	}
}
