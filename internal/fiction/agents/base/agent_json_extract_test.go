package base

import "testing"

func TestExtractJSONFromMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "fenced json block",
			in:   "Here you go:\n```json\n{\"a\":1}\n```\nThanks",
			want: `{"a":1}`,
		},
		{
			name: "unclosed fence",
			in:   "```json\n{\"a\":1}",
			want: `{"a":1}`,
		},
		{
			name: "prose around bare object",
			in:   `prefix {"a":{"b":2}} suffix`,
			want: `{"a":{"b":2}}`,
		},
		{
			name: "brace inside string does not close early",
			in:   `x {"note":"a } b","n":1} y`,
			want: `{"note":"a } b","n":1}`,
		},
		{
			name: "missing closing brace returns the remainder",
			in:   `prefix {"a":1`,
			want: `{"a":1`,
		},
		{
			name: "no object at all returns input unchanged",
			in:   "no json here",
			want: "no json here",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractJSONFromMarkdown(c.in); got != c.want {
				t.Fatalf("extractJSONFromMarkdown(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
