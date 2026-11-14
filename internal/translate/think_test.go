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
	var out, thoughts strings.Builder
	for _, ch := range chunks {
		v, th := f.Write(ch)
		out.WriteString(v)
		thoughts.WriteString(th)
	}
	v, th := f.Flush()
	out.WriteString(v)
	thoughts.WriteString(th)

	if got := out.String(); got != "Hi  there" {
		t.Fatalf("visible stream = %q, want %q", got, "Hi  there")
	}
	if got := thoughts.String(); got != "the user said hi" {
		t.Fatalf("thinking stream = %q, want %q", got, "the user said hi")
	}
}

func TestThinkFilterUnterminated(t *testing.T) {
	o, _ := tags()
	f := NewThinkFilter()
	v, th := f.Write("answer " + o + "still going")
	v2, th2 := f.Flush()
	visible := v + v2
	thought := th + th2

	if visible != "answer " {
		t.Fatalf("visible = %q, want %q", visible, "answer ")
	}
	if thought != "still going" {
		t.Fatalf("thought = %q, want %q", thought, "still going")
	}
}
