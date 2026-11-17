package states

import "strconv"

// An Arc is a transition of a diagram that Draw draws: the indexes of its two
// states and its label.
type Arc struct {
	From, To int
	Label    string
}

// Draw returns the SVG diagram of the states nodes, in order, and the arcs
// between them, in the form of the diagram of a Go machine. A state whose
// entry in first is true is a first state.
func Draw(nodes []string, arcs []Arc, first []bool) []byte {
	keys := make([]string, len(nodes))
	initial := map[string]bool{}
	for i := range nodes {
		keys[i] = strconv.Itoa(i)
		if i < len(first) && first[i] {
			initial[keys[i]] = true
		}
	}
	as := make([]arc, 0, len(arcs))
	for _, a := range arcs {
		as = append(as, arc{from: a.From, to: a.To, label: a.Label})
	}
	return stateSVG(nodes, as, initial, keys)
}
