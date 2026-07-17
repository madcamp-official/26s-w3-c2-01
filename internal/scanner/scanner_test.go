package scanner

import (
	"errors"
	"testing"
)

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr bool
	}{
		{name: "valid", options: Options{Roots: []string{"D:\\Projects"}, MaxDepth: 20}},
		{name: "missing roots", options: Options{MaxDepth: 20}, wantErr: true},
		{name: "invalid depth", options: Options{Roots: []string{"D:\\Projects"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIssueWrapsCause(t *testing.T) {
	cause := errors.New("access denied")
	issue := Issue{Path: `C:\System Volume Information`, Operation: "read", Err: cause}

	if !errors.Is(issue, cause) {
		t.Fatal("Issue must wrap its cause")
	}
}
