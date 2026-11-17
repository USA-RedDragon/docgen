package gen

import (
	"context"
	"fmt"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// valueFalse is the false value of a boolean argument of a marker.
const valueFalse = "false"

const (
	argFeatures  = "features"
	argAllFeat   = "allfeatures"
	argNoDefault = "nodefaultfeatures"
)

// featureArgs holds the features=, allfeatures= and nodefaultfeatures=
// arguments of a Rust marker that runs cargo. Each one replaces the matching
// setting for the marker.
type featureArgs struct {
	list      []string
	hasList   bool
	all       bool
	hasAll    bool
	noDefault bool
	hasNoDef  bool
	count     int
}

// bind reads the three arguments of a.
func (f *featureArgs) bind(a *marker.Args) {
	if v, ok := a.Optional(argFeatures); ok {
		f.count++
		a.NonEmpty(argFeatures)
		list, err := cargo.SplitFeatures(v)
		if err != nil {
			a.Fail(argFeatures + "= " + err.Error())
		}
		f.list, f.hasList = list, true
	}
	f.all, f.hasAll = featureBool(a, argAllFeat, &f.count)
	f.noDefault, f.hasNoDef = featureBool(a, argNoDefault, &f.count)
}

// featureBool reads the argument key, which is true or false, and counts it.
func featureBool(a *marker.Args, key string, count *int) (value, has bool) {
	v, ok := a.Optional(key)
	if !ok {
		return false, false
	}
	*count++
	switch v {
	case "true":
		return true, true
	case valueFalse:
		return false, true
	}
	a.Fail(key + "= is not true or false")
	return false, false
}

// resolve returns the features of a marker: base, the features setting, with
// the arguments of the marker in place of the matching settings.
func (f *featureArgs) resolve(base cargo.Features) cargo.Features {
	if f.hasList {
		base.List = f.list
	}
	if f.hasAll {
		base.All = f.all
	}
	if f.hasNoDef {
		base.NoDefault = f.noDefault
	}
	return base
}

// features returns the features of a marker with the arguments f, from the
// features setting of g.
func (g *Generator) features(f *featureArgs) cargo.Features {
	return f.resolve(g.rust.features)
}

// featureFlags returns the cargo flags that enable feats for the crate. An
// entry that names no crate or feature is an error.
func (g *Generator) featureFlags(ctx context.Context, crate string, feats cargo.Features) ([]string, error) {
	ws, pkg, err := g.cargoPackage(ctx, crate)
	if err != nil {
		return nil, err
	}
	if err := feats.Validate(ws); err != nil {
		return nil, fmt.Errorf("crate %s: %w", crate, err)
	}
	return feats.Args(pkg), nil
}
