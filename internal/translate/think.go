package translate

import "strings"

const (
	thinkOpen  = "<" + "think" + ">"
	thinkClose = "<" + "/" + "think" + ">"
)

// StripThink removes think blocks from a complete string. Kept for callers that
// only want the visible answer.
func StripThink(s string) string {
	visible, _ := SplitThink(s)
	return visible
}

// SplitThink separates a complete string into visible text and thinking text.
func SplitThink(s string) (visible, thought string) {
	var v, th strings.Builder
	for {
		i := strings.Index(s, thinkOpen)
		if i < 0 {
			v.WriteString(s)
			return v.String(), th.String()
		}
		v.WriteString(s[:i])
		rest := s[i+len(thinkOpen):]
		j := strings.Index(rest, thinkClose)
		if j < 0 {
			th.WriteString(rest)
			return v.String(), th.String()
		}
		th.WriteString(rest[:j])
		s = rest[j+len(thinkClose):]
	}
}

// ThinkFilter splits a streamed content feed into visible text and thinking
// text, where a tag can be split across chunks.
type ThinkFilter struct {
	inThink bool
	buf     string
}

// NewThinkFilter returns an empty filter.
func NewThinkFilter() *ThinkFilter { return &ThinkFilter{} }

// Write consumes one delta and returns text that may be emitted, split into the
// visible answer and the thinking block.
func (f *ThinkFilter) Write(chunk string) (visible, thought string) {
	if chunk == "" {
		return "", ""
	}
	f.buf += chunk

	var v, th strings.Builder
	for {
		if !f.inThink {
			i := strings.Index(f.buf, thinkOpen)
			if i < 0 {
				hold := partialSuffix(f.buf, thinkOpen)
				v.WriteString(f.buf[:len(f.buf)-hold])
				f.buf = f.buf[len(f.buf)-hold:]
				return v.String(), th.String()
			}
			v.WriteString(f.buf[:i])
			f.buf = f.buf[i+len(thinkOpen):]
			f.inThink = true
			continue
		}

		j := strings.Index(f.buf, thinkClose)
		if j < 0 {
			hold := partialSuffix(f.buf, thinkClose)
			if len(f.buf) > hold {
				th.WriteString(f.buf[:len(f.buf)-hold])
			}
			f.buf = f.buf[len(f.buf)-hold:]
			return v.String(), th.String()
		}
		th.WriteString(f.buf[:j])
		f.buf = f.buf[j+len(thinkClose):]
		f.inThink = false
	}
}

// Flush returns any buffered text once the stream has ended.
func (f *ThinkFilter) Flush() (visible, thought string) {
	rest := f.buf
	f.buf = ""
	if f.inThink {
		return "", rest
	}
	return rest, ""
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
