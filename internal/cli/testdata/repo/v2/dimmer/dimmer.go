// Package dimmer is a lamp with a brightness level, for the docgen:config
// examples.
package dimmer

import (
	"time"

	"github.com/USA-RedDragon/docgen/v2/machine"
)

// Level is a brightness level.
type Level uint8

// The Level values.
const (
	// LevelOff gives no light.
	LevelOff Level = iota
	// LevelLow gives some light.
	LevelLow
	// LevelHigh gives full light.
	LevelHigh
)

// DefaultColor is the RGB color of a new Dimmer.
const DefaultColor = 0xffaa00

// Look is the light of a Dimmer.
type Look struct {
	// Level is the brightness at start.
	Level Level
	// Color is an RGB color.
	Color uint32
}

// NewLook returns l with its defaults.
func NewLook(l Look) Look {
	if l.Level == LevelOff {
		l.Level = LevelLow + 1
	}
	if l.Color == 0 {
		l.Color = DefaultColor
	}
	return l
}

// Steps sets how a Dimmer changes its level.
type Steps struct {
	// Step is the change of one press. 0 means 1.
	Step int
}

// NewSteps returns s with its defaults.
func NewSteps(s Steps) Steps {
	if s.Step < 1 {
		s.Step = 1
	}
	return s
}

// Tag names a Dimmer.
type Tag struct {
	// Name is a label next to the Dimmer.
	Name string
}

// Timing holds the times of a Dimmer.
type Timing struct {
	// Fade is the time of a change of level.
	Fade time.Duration
}

type counts struct {
	// Presses is the number of presses.
	Presses int
}

// Setup holds all the settings of a Dimmer.
type Setup struct {
	// Timing holds the times.
	Timing
	counts
	// Look is the light at start.
	Look Look
}

// Group holds the lamps of a room.
type Group struct {
	// Main is the first lamp.
	Main *machine.Lamp
	// Spares are the other lamps.
	Spares []machine.Lamp
	// States holds the state of each lamp by name.
	States map[string]machine.LampState
}

// Range limits the level.
type Range struct {
	Min Level // Min is the lowest level.
	// Max is the highest level.
	Max Level // This comment is not in the table.
}
