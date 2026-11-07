// Package door is a state machine with its own tests, for the run= argument.
package door

// State is the state of a Door.
type State uint8

// The State values.
const (
	// StateClosed is shut.
	StateClosed State = iota
	// StateOpen is open.
	StateOpen
)

// Door opens and closes.
type Door struct{ state State }

// State returns the state of the Door.
func (d *Door) State() State { return d.state }

// Open opens the Door.
func (d *Door) Open() { d.state = StateOpen }

// Close closes the Door.
func (d *Door) Close() { d.state = StateClosed }
