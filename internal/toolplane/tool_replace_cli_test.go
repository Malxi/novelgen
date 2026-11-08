package toolplane

import "testing"

func TestResolveRecapChapterID(t *testing.T) {
	tests := []struct {
		name    string
		bodyID  string
		flagID  string
		want    string
		wantErr bool
	}{
		{name: "body only", bodyID: "P1-V1-C1", want: "P1-V1-C1"},
		{name: "flag fills empty body", flagID: "P1-V1-C2", want: "P1-V1-C2"},
		{name: "matching ids", bodyID: "P1-V1-C1", flagID: "P1-V1-C1", want: "P1-V1-C1"},
		{name: "mismatched ids", bodyID: "P1-V1-C1", flagID: "P1-V1-C2", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveRecapChapterID(test.bodyID, test.flagID)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got id %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("id = %q, want %q", got, test.want)
			}
		})
	}
}
