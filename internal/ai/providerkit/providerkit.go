// Package providerkit holds the HTTP, Responses API, attachment, and citation
// plumbing shared by provider clients. It must not import a concrete provider.
package providerkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"hark/internal/ai"

	"golang.org/x/sys/unix"
)

const (
	MaxImageAttachments          = 4
	MaxImageAttachmentBytes      = 20 << 20
	MaxTotalImageAttachmentBytes = 40 << 20
)

// NewHTTPClient returns a client with explicit timeouts and no redirect
// following, so a provider cannot bounce a request carrying the API key
// somewhere else.
func NewHTTPClient(providerName string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("%s API redirects are not allowed", providerName)
		},
	}
}

// SendEvent reports whether the event was delivered; false means the caller
// should stop streaming.
func SendEvent(ctx context.Context, events chan<- ai.Event, event ai.Event) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

// EachImageAttachment validates every attachment and hands the resulting data
// URL to add, enforcing the per-image and total size budgets.
func EachImageAttachment(attachments []ai.Attachment, add func(dataURL string) error) error {
	var totalBytes int64
	for index, attachment := range attachments {
		if attachment.Type != "image" {
			return fmt.Errorf("unsupported attachment type %q", attachment.Type)
		}
		if index >= MaxImageAttachments {
			return fmt.Errorf("too many image attachments: maximum is %d", MaxImageAttachments)
		}

		dataURL, size, err := ImageDataURL(attachment.Path, attachment.MIMEType, MaxTotalImageAttachmentBytes-totalBytes)
		if err != nil {
			return err
		}
		totalBytes += size

		if err := add(dataURL); err != nil {
			return err
		}
	}
	return nil
}

// ImageDataURL reads a regular image file and encodes it as a data URL. The
// content type is detected from the bytes, not trusted from the request.
func ImageDataURL(path, requestedMIMEType string, remainingTotalBudget int64) (string, int64, error) {
	if strings.TrimSpace(path) == "" {
		return "", 0, errors.New("image attachment path must not be empty")
	}

	file, err := openRegularFile(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("inspect image attachment %q: %w", path, err)
	}
	if info.Size() > MaxImageAttachmentBytes {
		return "", 0, fmt.Errorf("image attachment %q exceeds size limit of %d bytes", path, MaxImageAttachmentBytes)
	}
	if info.Size() > remainingTotalBudget {
		return "", 0, fmt.Errorf("image attachments exceed total size limit of %d bytes", MaxTotalImageAttachmentBytes)
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxImageAttachmentBytes+1))
	if err != nil {
		return "", 0, fmt.Errorf("read image attachment %q: %w", path, err)
	}
	if len(data) > MaxImageAttachmentBytes {
		return "", 0, fmt.Errorf("image attachment %q exceeds size limit of %d bytes", path, MaxImageAttachmentBytes)
	}

	detectedMIMEType := http.DetectContentType(data)
	switch detectedMIMEType {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return "", 0, fmt.Errorf("image attachment %q has unsupported content type %q", path, detectedMIMEType)
	}
	requestedMIMEType = strings.ToLower(strings.TrimSpace(requestedMIMEType))
	if requestedMIMEType != "" && requestedMIMEType != detectedMIMEType {
		return "", 0, fmt.Errorf("image attachment %q content type is %q, not %q", path, detectedMIMEType, requestedMIMEType)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:" + detectedMIMEType + ";base64," + encoded, int64(len(data)), nil
}

func openRegularFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ELOOP) {
		return nil, fmt.Errorf("image attachment %q must be a regular file, not a symlink", path)
	}
	if err != nil {
		return nil, fmt.Errorf("open image attachment %q: %w", path, err)
	}

	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open image attachment %q: invalid file descriptor", path)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect image attachment %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("image attachment %q must be a regular file", path)
	}
	return file, nil
}

// Citation is a provider-neutral URL citation covering a rune range of the
// answer text.
type Citation struct {
	Title      string
	URL        string
	StartIndex int
	EndIndex   int
}

// FormatCitedAnswer rewrites cited spans as numbered Markdown links and appends
// a source list. Citations with unusable URLs or ranges are dropped. Consulted
// sources are only listed when nothing was cited inline.
func FormatCitedAnswer(answer string, citations []Citation, consulted []ai.Source) (string, []ai.Source) {
	type indexedCitation struct {
		Citation
		sourceNumber int
	}
	type candidateCitation struct {
		Citation
		originalIndex int
	}

	sources := make([]ai.Source, 0, len(citations)+len(consulted))
	sourceNumberByURL := make(map[string]int, len(citations)+len(consulted))
	addSource := func(source ai.Source) int {
		source.URL = SafeWebURL(source.URL)
		if source.URL == "" {
			return 0
		}
		if number := sourceNumberByURL[source.URL]; number > 0 {
			if sources[number-1].Title == "" && source.Title != "" {
				sources[number-1].Title = source.Title
			}
			return number
		}
		sources = append(sources, source)
		number := len(sources)
		sourceNumberByURL[source.URL] = number
		return number
	}

	runes := []rune(answer)
	candidates := make([]candidateCitation, 0, len(citations))
	for index, citation := range citations {
		candidates = append(candidates, candidateCitation{Citation: citation, originalIndex: index})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].StartIndex > candidates[j].StartIndex
	})
	accepted := make([]bool, len(citations))
	lastStart := len(runes) + 1
	for _, citation := range candidates {
		if citation.StartIndex < 0 || citation.EndIndex <= citation.StartIndex || citation.EndIndex > len(runes) || citation.EndIndex > lastStart {
			continue
		}
		accepted[citation.originalIndex] = true
		lastStart = citation.StartIndex
	}

	indexed := make([]indexedCitation, 0, len(citations))
	for index, citation := range citations {
		if !accepted[index] {
			continue
		}
		number := addSource(ai.Source{Title: citation.Title, URL: citation.URL})
		if number == 0 {
			continue
		}
		citation.URL = sources[number-1].URL
		indexed = append(indexed, indexedCitation{Citation: citation, sourceNumber: number})
	}
	if len(sources) == 0 {
		for _, source := range consulted {
			addSource(source)
		}
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		return indexed[i].StartIndex > indexed[j].StartIndex
	})

	for _, citation := range indexed {
		reference := []rune(fmt.Sprintf("[[%d]](%s)", citation.sourceNumber, markdownURL(citation.URL)))
		runes = append(append(append([]rune{}, runes[:citation.StartIndex]...), reference...), runes[citation.EndIndex:]...)
	}

	if len(sources) == 0 {
		return string(runes), nil
	}

	var formatted strings.Builder
	formatted.WriteString(strings.TrimSpace(string(runes)))
	formatted.WriteString("\n\nSources: ")
	for index, source := range sources {
		if index > 0 {
			formatted.WriteString(" · ")
		}
		title := markdownTitle(source.Title)
		if title == "" {
			title = sourceHost(source.URL)
		}
		fmt.Fprintf(&formatted, "[[%d]](%s) %s", index+1, markdownURL(source.URL), title)
	}
	return strings.TrimSpace(formatted.String()), sources
}

// SafeWebURL returns value when it is an absolute http or https URL, and the
// empty string otherwise. The overlay re-validates before opening a link.
func SafeWebURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func sourceHost(value string) string {
	parsed, err := url.Parse(value)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "Source"
}

func markdownTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, "[", "(")
	return strings.ReplaceAll(value, "]", ")")
}

func markdownURL(value string) string {
	value = strings.ReplaceAll(value, "(", "%28")
	value = strings.ReplaceAll(value, ")", "%29")
	return strings.ReplaceAll(value, " ", "%20")
}

// APIError builds an error from a non-2xx provider response, preferring the
// structured message the provider returns.
func APIError(providerName string, resp *http.Response) error {
	limited := io.LimitReader(resp.Body, 64*1024)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("%s request failed with HTTP %d and unreadable body: %w", providerName, resp.StatusCode, err)
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err == nil && body.Error.Message != "" {
		return fmt.Errorf("%s request failed with HTTP %d: %s", providerName, resp.StatusCode, body.Error.Message)
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		text = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("%s request failed with HTTP %d: %s", providerName, resp.StatusCode, text)
}
