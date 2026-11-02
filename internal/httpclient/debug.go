package httpclient

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/text"
)

// LogBody logs a request body. If the body is a kind of text (from the
// Content-Type of headers), all its lines are logged, otherwise its first 64
// bytes. This function doesn't manage compression: for a response, use
// Response.LogBody. If debug is true, lines are logged with Debug, otherwise
// with Info.
func LogBody(body []byte, headers HeaderVec, debug bool, l *logger.Logger) {
	if ct, ok := headers.ContentType(); ok && !IsKindOfText(ct) {
		LogBytes(body, 64, debug, l)
		return
	}
	charset, err := headers.CharacterEncoding()
	if err != nil {
		LogBytes(body, 64, debug, l)
		return
	}
	s, ok := charset.Decode(body)
	if !ok {
		LogBytes(body, 64, debug, l)
		return
	}
	LogText(s, debug, l)
}

// LogText logs text, line by line.
func LogText(s string, debug bool, l *logger.Logger) {
	log := l.Info
	if debug {
		log = l.Debug
	}
	if s == "" {
		log("")
		return
	}
	for line := range strings.SplitSeq(s, "\n") {
		log(line)
	}
}

// LogBytes logs at most max bytes.
func LogBytes(b []byte, max int, debug bool, l *logger.Logger) {
	b = b[:min(len(b), max)]
	s := ""
	if len(b) > 0 {
		s = fmt.Sprintf("Bytes <%s...>", hex.EncodeToString(b))
	}
	if debug {
		l.Debug(s)
	} else {
		l.Info(s)
	}
}

// LogBody logs the response body as text if possible, or its first 64
// bytes.
func (r *Response) LogBody(debug bool, l *logger.Logger) {
	if ct, ok := r.Headers.ContentType(); ok && !IsKindOfText(ct) {
		LogBytes(r.Body, 64, debug, l)
		return
	}
	s, err := r.Text()
	if err != nil {
		LogBytes(r.Body, 64, debug, l)
		return
	}
	LogText(s, debug, l)
}

func (r *Response) LogInfoAll(l *logger.Logger) {
	l.Info(r.GetStatusLineHeaders(l.Color))
	r.LogBody(false, l)
	l.Info("")
}

// GetStatusLineHeaders returns the status line and headers of this response.
func (r *Response) GetStatusLineHeaders(color bool) string {
	var s text.StyledString
	s.PushWith(fmt.Sprintf("%s %d\n", r.Version, r.Status), text.NewStyle().Green().Bold())
	for _, h := range r.Headers.All() {
		s.PushWith(h.Name, text.NewStyle().Cyan().Bold())
		s.Push(": " + h.Value + "\n")
	}
	if color {
		return s.Render(text.FormatANSI)
	}
	return s.Render(text.FormatPlain)
}

// Timings is the timing information of an HTTP transfer (see
// https://hurl.dev/docs/response.html#timings). Durations are measured from
// the start of the transfer, like libcurl's.
type Timings struct {
	BeginCall     time.Time
	EndCall       time.Time
	NameLookup    time.Duration
	Connect       time.Duration
	AppConnect    time.Duration
	PreTransfer   time.Duration
	StartTransfer time.Duration
	Total         time.Duration
}

// Log logs these timings.
func (t Timings) Log(l *logger.Logger) {
	l.DebugImportant("Timings:")
	l.Debug("begin: " + FormatUtc(t.BeginCall))
	l.Debug("end: " + FormatUtc(t.EndCall))
	l.Debug(fmt.Sprintf("namelookup: %d µs", t.NameLookup.Microseconds()))
	l.Debug(fmt.Sprintf("connect: %d µs", t.Connect.Microseconds()))
	l.Debug(fmt.Sprintf("app_connect: %d µs", t.AppConnect.Microseconds()))
	l.Debug(fmt.Sprintf("pre_transfer: %d µs", t.PreTransfer.Microseconds()))
	l.Debug(fmt.Sprintf("start_transfer: %d µs", t.StartTransfer.Microseconds()))
	l.Debug(fmt.Sprintf("total: %d µs", t.Total.Microseconds()))
}

// FormatUtc formats t like chrono's DateTime<Utc> Display:
// "2006-01-02 15:04:05.123456 UTC", with 0, 3, 6 or 9 fractional digits.
func FormatUtc(t time.Time) string {
	t = t.UTC()
	s := t.Format("2006-01-02 15:04:05")
	ns := t.Nanosecond()
	switch {
	case ns == 0:
	case ns%1_000_000 == 0:
		s += fmt.Sprintf(".%03d", ns/1_000_000)
	case ns%1_000 == 0:
		s += fmt.Sprintf(".%06d", ns/1_000)
	default:
		s += fmt.Sprintf(".%09d", ns)
	}
	return s + " UTC"
}

// Call holds an HTTP request and its response.
type Call struct {
	// Request is the real HTTP request (vs the request spec).
	Request Request
	// Response is the real HTTP response.
	Response Response
	Timings  Timings
}
