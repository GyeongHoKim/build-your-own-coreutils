package rm

import (
	"path/filepath"
	"testing"
)

func BenchmarkRemoveTree(b *testing.B) {
	for _, bm := range []struct {
		name string
		jobs int
	}{
		{"jobs=1", 1},
		{"jobs=default", 0},
	} {
		b.Run(bm.name, func(b *testing.B) {
			root := filepath.Join(b.TempDir(), "t")
			for range b.N {
				b.StopTimer()
				makeTree(b, root, 100, 100)
				b.StartTimer()
				if !Remove([]string{root}, Options{Recursive: true, Jobs: bm.jobs}, Callbacks{}) {
					b.Fatal("Remove reported failure")
				}
			}
		})
	}
}
