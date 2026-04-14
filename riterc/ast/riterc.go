package ast

import (
	"cmp"
	"maps"

	"github.com/Masterminds/semver/v3"
)

type Riterc struct {
	Version      *semver.Version `yaml:"version"`
	Verbose      *bool           `yaml:"verbose"`
	Silent       *bool           `yaml:"silent"`
	Color        *bool           `yaml:"color"`
	DisableFuzzy *bool           `yaml:"disable-fuzzy"`
	Concurrency  *int            `yaml:"concurrency"`
	Interactive  *bool           `yaml:"interactive"`
	Failfast     bool            `yaml:"failfast"`
	Experiments  map[string]int  `yaml:"experiments"`
}

// Merge combines the current Riterc with another Riterc, prioritizing non-nil fields from the other Riterc.
func (t *Riterc) Merge(other *Riterc) {
	if other == nil {
		return
	}

	t.Version = cmp.Or(other.Version, t.Version)

	if t.Experiments == nil && other.Experiments != nil {
		t.Experiments = other.Experiments
	} else if t.Experiments != nil && other.Experiments != nil {
		maps.Copy(t.Experiments, other.Experiments)
	}

	t.Verbose = cmp.Or(other.Verbose, t.Verbose)
	t.Silent = cmp.Or(other.Silent, t.Silent)
	t.Color = cmp.Or(other.Color, t.Color)
	t.DisableFuzzy = cmp.Or(other.DisableFuzzy, t.DisableFuzzy)
	t.Concurrency = cmp.Or(other.Concurrency, t.Concurrency)
	t.Interactive = cmp.Or(other.Interactive, t.Interactive)
	t.Failfast = cmp.Or(other.Failfast, t.Failfast)
}
