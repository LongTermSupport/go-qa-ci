package docs

import (
	"context"
	"fmt"
	"sync"

	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/stagerun"
)

// stageRules remembers what the rules commands of the project stages printed, so that resolving
// many identifiers runs them once.
type stageRules struct {
	once  sync.Once
	rules []stage.Rule
	err   error
}

// stageRules are the rules every stage's rules command prints, in stage order. A command that
// fails is an error: an identifier must never be called dangling because a command did not run.
func (r Resolver) stageRules() ([]stage.Rule, error) {
	if len(r.Stages) == 0 {
		return nil, nil
	}
	if r.rules == nil {
		return loadStageRules(r.Root, r.Stages)
	}
	r.rules.once.Do(func() { r.rules.rules, r.rules.err = loadStageRules(r.Root, r.Stages) })
	return r.rules.rules, r.rules.err
}

func loadStageRules(root string, stages []stage.Stage) ([]stage.Rule, error) {
	var all []stage.Rule
	for _, s := range stages {
		rules, err := stagerun.LoadRules(context.Background(), root, s)
		if err != nil {
			return nil, fmt.Errorf("reading the rules of the project stages: %w", err)
		}
		all = append(all, rules...)
	}
	return all, nil
}

// stageIdentifiers are the identifiers the project's stages make: each stage's name and every
// rule it prints.
func (r Resolver) stageIdentifiers() ([]string, error) {
	rules, err := r.stageRules()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(r.Stages)+len(rules))
	for _, s := range r.Stages {
		ids = append(ids, s.Name)
	}
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids, nil
}
