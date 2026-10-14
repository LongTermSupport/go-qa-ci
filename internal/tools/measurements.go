package tools

import (
	"io"
	"slices"
	"sync"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

// Measurements carry the coverage the test step measured to the coverage step, so one go test run
// (race detector, shuffled, uncached, counting statements) feeds both verdicts and the coverage
// figure describes the very run the test step vouched for.
type Measurements struct {
	mu       sync.Mutex
	byModule map[string]runners.Coverage
}

// NewMeasurements returns an empty store.
func NewMeasurements() *Measurements {
	return &Measurements{byModule: map[string]runners.Coverage{}}
}

func (m *Measurements) put(module string, cov runners.Coverage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byModule[module] = cov
}

// take returns the coverage measured for a module, if the test step ran it and the tests passed.
func (m *Measurements) take(module string) (runners.Coverage, bool) {
	if m == nil {
		return runners.Coverage{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cov, ok := m.byModule[module]
	return cov, ok
}

// NewExecutor builds the executor of a run. When the run selects both the test and the coverage
// step, the two share one test run.
func NewExecutor(ctx Context, stderr io.Writer) Executor {
	e := Executor{Ctx: ctx, Stderr: stderr}
	if slices.Contains(ctx.Selected, "test") && slices.Contains(ctx.Selected, "coverage") {
		e.Shared = NewMeasurements()
	}
	return e
}
