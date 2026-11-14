package translate

import "strings"

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// StripThink removes <think>…</think> blocks from a complete string. Some
// chat templates inline chain-of-thought into the content stream instead of
// using a separate reasoning field.
func StripThink(s string) string {
	for {
		i := strings.Index(s, thinkOpen)
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], thinkClose)
		if j < 0 {
			// Unterminated block: drop the rest.
			return s[:i]
		}
		s = s[:i] + s[i+j+len(thinkClose):]
	}
}

// ThinkFilter strips inline <think> blocks from a streamed content feed, where
// a tag can be split across chunks. Call Write for every delta and Flush at the
// end; both return text that is safe to forward to the client.
type ThinkFilter struct {
	inThink bool
	buf     string
}

// NewThinkFilter returns an empty filter.
func NewThinkFilter() *ThinkFilter { return &ThinkFilter{} }

// Write consumes one delta and returns the text that may be emitted.
func (f *ThinkFilter) Write(chunk string) string {
	if chunk == "" {
		return ""
	}
	f.buf += chunk

	var out strings.Builder
	for {
		if !f.inThink {
			i := strings.Index(f.buf, thinkOpen)
			if i < 0 {
				// Hold back a possible partial "<think" prefix.
				hold := partialSuffix(f.buf, thinkOpen)
				out.WriteString(f.buf[:len(f.buf)-hold])
				f.buf = f.buf[len(f.buf)-hold:]
				return out.String()
			}
			out.WriteString(f.buf[:i])
			f.buf = f.buf[i+len(thinkOpen):]
			f.inThink = true
			continue
		}

		j := strings.Index(f.buf, thinkClose)
		if j < 0 {
			// Still thinking: keep only a tail that could hold a partial close tag.
			hold := partialSuffix(f.buf, thinkClose)
			f.buf = f.buf[len(f.buf)-hold:]
			return out.String()
		}
		f.buf = f.buf[j+len(thinkClose):]
		f.inThink = false
	}
}

// Flush returns any buffered text once the stream has ended.
func (f *ThinkFilter) Flush() string {
	rest := f.buf
	f.buf = ""
	if f.inThink {
		// Unterminated think block at EOF: nothing safe to emit.
		return ""
	}
	return rest
}

// partialSuffix returns the length of the longest suffix of s that is a proper
// prefix of tag, so a tag split across chunks is not emitted as text.
func partialSuffix(s, tag string) int {
	max := len(tag) - 1
	if len(s) < max {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}
