package touch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateNoCreateLexicalDotDot(t *testing.T) {
	root := t.TempDir()
	writeSeeded(t, filepath.Join(root, "f"), "")
	t.Chdir(root)

	tests := []struct {
		path    string
		wantErr bool
	}{
		{`f\..\missing`, false},
		{`missing\..\absent`, false},
		{`missing\..\f\x`, true},
		{root + `\missing\..\f\x`, true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := Update(tt.path, requestTimes, Options{NoCreate: true})
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got error %v, want error %t", err, tt.wantErr)
			}
		})
	}
	if _, err := os.Lstat("missing"); err == nil {
		t.Error("missing was created")
	}
}
