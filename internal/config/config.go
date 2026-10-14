// Package config loads qa/go-qa.toml: built-in defaults overridden by the project file, with
// unknown keys rejected so that a misspelt setting can never silently do nothing.
package config

import (
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

// DefaultPath is the project configuration file, relative to the root.
const DefaultPath = "qa/go-qa.toml"

// Config is the effective configuration after the defaults are overridden. The README
// "Configuration" section states every default; a test keeps the two equal.
type Config struct {
	// Record is the project record location and its anti-bloat policy: `record = "<path>"` or a
	// [record] table.
	Record RecordPolicy `toml:"record"`
	// DocsDir holds project-authored rule pages (<id>.md).
	DocsDir string `toml:"docs_dir"`
	// ScanExclude lists globs, relative to the root, excluded from the source scans.
	ScanExclude []string `toml:"scan_exclude"`
	// Modules lists further Go modules, not part of a go.work, checked in their own directory.
	Modules []string `toml:"modules"`
	// Summary configures the generated agent summary region.
	Summary Summary `toml:"summary"`
	// Tools holds one switch per tool.
	Tools Tools `toml:"tools"`
	// AstGrep configures the ast-grep detector.
	AstGrep AstGrep `toml:"ast_grep"`
	// Analyzers configures the analyzers detector.
	Analyzers Analyzers `toml:"analyzers"`
	// Coverage configures the coverage gate.
	Coverage Coverage `toml:"coverage"`
	// Mutants configures mutation testing.
	Mutants Mutants `toml:"mutants"`
	// Stages are the project's own defences and runners, run in the phase each names.
	Stages []stage.Stage `toml:"stage"`
	// Resolvers document identifiers no listing names, such as the codes of a linter a stage runs.
	Resolvers []Resolver `toml:"resolver"`
}

// Resolver is a [[resolver]] entry: the documentation of every identifier matching a glob.
type Resolver struct {
	// Matches is a glob over identifiers.
	Matches string `toml:"matches"`
	// Text is the documentation; {id} stands for the identifier.
	Text string `toml:"text"`
}

// Analyzers configures the analyzers detector.
type Analyzers struct {
	// Driver is the project's analyzer driver: a package pattern inside Module (the root when
	// Module is empty), "./..." style, whose main calls driver.Main with the project's analyzers.
	// Empty means go-qa itself is the vet tool, with the analyzers bundled in it.
	Driver string `toml:"driver"`
	// Module is the directory, relative to the root, of the Go module holding the driver. Set it
	// when the project's rules live in a nested module, so the dependencies they bring stay out of
	// the go.mod of the project's own binaries. Empty means the driver is in the root module.
	Module string `toml:"module"`
}

// BuildDir is the directory the driver is built in: the module's, or the root.
func (a Analyzers) BuildDir(root string) string {
	if a.Module == "" {
		return root
	}
	return filepath.Join(root, filepath.FromSlash(a.Module))
}

// RecordPolicy is the [record] table: where the project record lives and how large it may grow.
type RecordPolicy struct {
	// Path of the project record, relative to the root.
	Path string `toml:"path"`
	// MaxTotal caps the number of exceptions.
	MaxTotal int `toml:"max_total"`
	// MaxPerRule caps the exceptions for one rule id.
	MaxPerRule int `toml:"max_per_rule"`
	// MaxPolicy caps broad (glob) exceptions marked policy = true.
	MaxPolicy int `toml:"max_policy"`
	// MaxReviewDays is the longest decided_on to review_by gap, for any tier.
	MaxReviewDays int `toml:"max_review_days"`
	// Deciders are the people who sign the ratchet, mark entries human and co-sign.
	Deciders []string `toml:"deciders"`
	// HistoryBase is the git ref whose merge base supplies the previous record.
	HistoryBase string `toml:"history_base"`
	// Trust maps authoring models to tiers and sets each tier's limits.
	Trust Trust `toml:"trust"`
}

// Trust is the [record.trust] table.
type Trust struct {
	// Models maps a model id prefix to a tier.
	Models map[string]Tier `toml:"models"`
	// Human, High, Medium and Low are the per-tier limits.
	Human  TierLimits `toml:"human"`
	High   TierLimits `toml:"high"`
	Medium TierLimits `toml:"medium"`
	Low    TierLimits `toml:"low"`
	// Cosign lists rule categories that need a decider below a tier.
	Cosign []Cosign `toml:"cosign"`
}

// TierLimits are one tier's limits; nil falls back to the record-wide cap.
type TierLimits struct {
	// MaxReviewDays is the tier's review horizon in days.
	MaxReviewDays *int `toml:"max_review_days"`
	// MaxCount is the number of entries the tier may hold.
	MaxCount *int `toml:"max_count"`
}

// Cosign is a rule category whose exceptions need a human decider when written below MinTier.
type Cosign struct {
	// Rules are rule id globs.
	Rules []string `toml:"rules"`
	// MinTier is the lowest tier that may decide alone.
	MinTier Tier `toml:"min_tier"`
}

// Summary configures the agent summary region.
type Summary struct {
	// File receives the generated region; empty disables the summary check.
	File string `toml:"file"`
	// FixHeadings name the project-page section whose first paragraph the summary quotes.
	FixHeadings []string `toml:"fix_headings"`
}

// Tools holds one on/off switch per tool of the catalogue.
type Tools struct {
	Fmt         bool `toml:"fmt"`
	Record      bool `toml:"record"`
	Suppression bool `toml:"suppression"`
	Summary     bool `toml:"summary"`
	Docs        bool `toml:"docs"`
	AstGrep     bool `toml:"ast_grep"`
	Golangci    bool `toml:"golangci"`
	Analyzers   bool `toml:"analyzers"`
	Deadcode    bool `toml:"deadcode"`
	Vuln        bool `toml:"vuln"`
	Mod         bool `toml:"mod"`
	Test        bool `toml:"test"`
	Coverage    bool `toml:"coverage"`
	Mutants     bool `toml:"mutants"`
}

// AstGrep configures the ast-grep detector.
type AstGrep struct {
	// Bundled runs the bundled gqaci.ast.* pack beside the project's own rules.
	Bundled bool `toml:"bundled"`
}

// Coverage configures the coverage gate.
type Coverage struct {
	// FailUnderStatements is the minimum percentage of statements covered.
	FailUnderStatements int `toml:"fail_under_statements"`
}

// Mutants configures mutation testing.
type Mutants struct {
	// Mode selects which code is mutated.
	Mode MutantsMode `toml:"mode"`
	// Base is the git ref diff mode compares against.
	Base string `toml:"base"`
	// ThresholdEfficacy is the least percentage of mutants the tests must kill.
	ThresholdEfficacy int `toml:"threshold_efficacy"`
}

// MutantsMode selects which code is mutated.
type MutantsMode string

// The mutation modes.
const (
	MutantsOff  MutantsMode = "off"
	MutantsDiff MutantsMode = "diff"
	MutantsFull MutantsMode = "full"
)

// Valid reports whether m is one of the modes.
func (m MutantsMode) Valid() bool {
	return m == MutantsOff || m == MutantsDiff || m == MutantsFull
}

// Tier is a trust tier of the record.
type Tier string

// The trust tiers, from most to least trusted, and Unknown for an unmapped model.
const (
	TierHuman   Tier = "human"
	TierHigh    Tier = "high"
	TierMedium  Tier = "medium"
	TierLow     Tier = "low"
	TierUnknown Tier = "unknown"
)

// Valid reports whether t is one of the tiers.
func (t Tier) Valid() bool {
	switch t {
	case TierHuman, TierHigh, TierMedium, TierLow, TierUnknown:
		return true
	default:
		return false
	}
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Record: RecordPolicy{
			Path:          "qa/record.toml",
			MaxTotal:      10,
			MaxPerRule:    3,
			MaxPolicy:     0,
			MaxReviewDays: 180,
			Deciders:      []string{},
			HistoryBase:   "origin/main",
			Trust: Trust{
				Models: map[string]Tier{},
				High:   TierLimits{MaxReviewDays: intPtr(90)},
				Medium: TierLimits{MaxReviewDays: intPtr(30), MaxCount: intPtr(3)},
				Low:    TierLimits{MaxReviewDays: intPtr(14), MaxCount: intPtr(1)},
				Cosign: []Cosign{},
			},
		},
		DocsDir:     "docs/defences",
		ScanExclude: []string{},
		Modules:     []string{},
		Summary:     Summary{FixHeadings: []string{"How to fix correctly"}},
		Tools: Tools{
			Fmt: true, Record: true, Suppression: true, Summary: true, Docs: true, AstGrep: true,
			Golangci: true, Analyzers: true, Deadcode: true, Vuln: true, Mod: true, Test: true,
			Coverage: true, Mutants: true,
		},
		AstGrep:  AstGrep{Bundled: true},
		Coverage: Coverage{FailUnderStatements: 80},
		Mutants:  Mutants{Mode: MutantsOff, Base: "origin/main", ThresholdEfficacy: 100},
	}
}

func intPtr(n int) *int { return &n }
