package translate

import (
	"strings"
	"testing"
)

// Tags are assembled at runtime so the literal sequences cannot be mangled by
// editors or tooling that treats angle-bracket tags as markup.
func tags() (open, close string) {
	lt, gt := string(rune(60)), string(rune(62))
	return lt + "think" + gt, lt + "/think" + gt
}

func TestStripThink(t *testing.T) {
	o, c := tags()
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{o + "secret" + c + "visible", "visible"},
		{"a " + o + "x" + c + "b " + o + "y" + c + "c", "a b c"},
		{"start " + o + "never closed", "start "},
		{"no tags here", "no tags here"},
	}
	for _, tc := range cases {
		if got := StripThink(tc.in); got != tc.want {
			t.Errorf("StripThink(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Chat templates stream a tag across chunk boundaries, so the filter must hold
// back partial tags instead of emitting them as text.
func TestThinkFilterAcrossChunks(t *testing.T) {
	lt := string(rune(60))
	chunks := []string{"Hi " + lt + "th", "ink>the user said", " hi" + lt + "/thi", "nk> there"}
	f := NewThinkFilter()
	var out strings.Builder
	for _, ch := range chunks {
		out.WriteString(f.Write(ch))
	}
	out.WriteString(f.Flush())

	if got := out.String(); got != "Hi  there" {
		t.Fatalf("filtered stream = %q, want %q", got, "Hi  there")
	}
}

func TestThinkFilterUnterminated(t *testing.T) {
	o, _ := tags()
	f := NewThinkFilter()
	var out strings.Builder
	out.WriteString(f.Write("answer " + o + "still going"))
	out.WriteString(f.Flush())

	if got := out.String(); got != "answer " {
		t.Fatalf("got %q, want %q", got, "answer ")
	}
}
