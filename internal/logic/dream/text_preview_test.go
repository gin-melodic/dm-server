package dream

import "testing"

func TestPreviewRunes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{
			name:  "short input",
			input: "梦见蛇",
			limit: 20,
			want:  "梦见蛇",
		},
		{
			name:  "unicode truncation",
			input: "我梦见一条很长的河流",
			limit: 4,
			want:  "我梦见一",
		},
		{
			name:  "zero limit",
			input: "梦境",
			limit: 0,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := previewRunes(tt.input, tt.limit); got != tt.want {
				t.Fatalf("previewRunes(%q, %d) = %q, want %q", tt.input, tt.limit, got, tt.want)
			}
		})
	}
}
