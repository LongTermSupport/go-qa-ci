package red

import "testing"

func TestNeverRuns(t *testing.T) {
	t.Skip("later") // want `unconditional t\.Skip`
	t.Fatal("unreachable in practice")
}

func TestNeverRunsF(t *testing.T) {
	t.Skipf("later %d", 1) // want `unconditional t\.Skipf`
}

func TestNeverRunsNow(t *testing.T) {
	t.SkipNow() // want `unconditional t\.SkipNow`
}

func BenchmarkNeverRuns(b *testing.B) {
	b.Skip("later") // want `unconditional b\.Skip`
}

func FuzzNeverRuns(f *testing.F) {
	f.Skip("later") // want `unconditional f\.Skip`
}

func TestSetupThenSkip(t *testing.T) {
	x := 1
	_ = x
	t.Skip("later") // want `unconditional t\.Skip`
}

func TestSubtestNeverRuns(t *testing.T) {
	t.Run("sub", func(t *testing.T) {
		t.Skip("later") // want `unconditional t\.Skip`
	})
}
