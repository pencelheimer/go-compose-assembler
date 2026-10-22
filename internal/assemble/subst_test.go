package assemble

import "testing"

func TestExpand(t *testing.T) {
	env := map[string]string{"SET": "v", "EMPTY": ""}
	for in, want := range map[string]string{
		"plain": "plain", "${SET}": "v", "${UNSET}": "", "a${SET}b": "avb", "$$x": "$x",
		"${UNSET:-d}": "d", "${EMPTY:-d}": "d", "${EMPTY-d}": "", "${UNSET-d}": "d", "${SET:-d}": "v",
		"${SET:?must be set}:80": "v:80", "${SET?a-b}": "v", "${UNSET:?m}": "",
		"${SET:+alt}": "alt", "${EMPTY:+alt}": "", "${EMPTY+alt}": "alt", "${UNSET+alt}": "", "${UNSET:-a-b}": "a-b",
		"${unterminated": "${unterminated",
	} {
		if got := expand(in, env); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
}
