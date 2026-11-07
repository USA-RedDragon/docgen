package states

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
)

type arc struct {
	from, to int
	label    string
}

const (
	charW   = 7.8
	boxH    = 34.0
	gap     = 46.0
	pad     = 16.0
	levelH  = 30.0
	baseH   = 34.0
	fontPx  = 13
	labelPx = 12
)

// A placedArc is an arc with its place in the diagram. lo and hi bound the arc
// and its label in x.
type placedArc struct {
	arc
	above      bool
	level      int
	x1, x2, lx float64
	lo, hi     float64
}

// stateSVG draws the states as a row of boxes in enum order, with each
// transition as an arc: arcs to a later state go above the row and arcs to
// an earlier state go below it. Arcs that overlap in x get different
// heights so that their labels do not overlap.
func stateSVG(nodes []string, arcs []arc, initial map[string]bool, keys []string) []byte {
	xs, ws, width := layoutBoxes(nodes)
	ps := placeArcs(arcs, xs)
	maxLevel := assignLevels(ps)
	minX, maxX := 0.0, width
	for _, p := range ps {
		minX = math.Min(minX, p.lo-pad)
		maxX = math.Max(maxX, p.hi+pad)
	}
	width = maxX - minX
	topSpace := arcSpace(maxLevel[true])
	rowY := topSpace
	height := topSpace + boxH + arcSpace(maxLevel[false])
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="%.0f 0 %.0f %.0f" font-family="Inconsolata, ui-monospace, Menlo, Consolas, monospace">`+"\n", width, height, minX, width, height)
	b.WriteString(`<defs><marker id="f" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="#2b5fa8"/></marker><marker id="b" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="#a0562a"/></marker></defs>` + "\n")
	fmt.Fprintf(&b, `<rect x="%.0f" width="%.0f" height="%.0f" fill="#ffffff"/>`+"\n", minX, width, height)
	for _, p := range ps {
		writeArc(&b, p, rowY)
	}
	for i, n := range nodes {
		sw, fill := "1.5", "#f3f5f9"
		if initial[keys[i]] {
			sw, fill = "3", "#e3ecf8"
		}
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.0f" rx="5" fill="%s" stroke="#1f2633" stroke-width="%s"/>`+"\n", xs[i]-ws[i]/2, rowY, ws[i], boxH, fill, sw)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="%d" text-anchor="middle" fill="#1f2633">%s</text>`+"\n", xs[i], rowY+boxH/2+4.5, fontPx, html.EscapeString(n))
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// layoutBoxes returns the center and the width of the box of each node, and the
// width of the row.
func layoutBoxes(nodes []string) (xs, ws []float64, width float64) {
	xs = make([]float64, len(nodes))
	ws = make([]float64, len(nodes))
	x := pad
	for i, n := range nodes {
		ws[i] = math.Max(96, float64(len(n))*charW+24)
		xs[i] = x + ws[i]/2
		x += ws[i] + gap
	}
	return xs, ws, x - gap + pad
}

// placeArcs returns each arc between two boxes with the centers xs, the shortest
// first. An arc from a box to itself has no place.
func placeArcs(arcs []arc, xs []float64) []*placedArc {
	var ps []*placedArc
	for _, a := range arcs {
		if a.from == a.to {
			continue
		}
		p := &placedArc{arc: a, above: a.to > a.from}
		p.x1, p.x2 = xs[a.from], xs[a.to]
		if p.above {
			p.x1 += 8
			p.x2 -= 8
		} else {
			p.x1 -= 8
			p.x2 += 8
		}
		p.lx = (p.x1 + p.x2) / 2
		lw := float64(len(a.label))*charW*float64(labelPx)/fontPx + 12
		p.lo = math.Min(math.Min(p.x1, p.x2), p.lx-lw/2)
		p.hi = math.Max(math.Max(p.x1, p.x2), p.lx+lw/2)
		ps = append(ps, p)
	}
	sort.SliceStable(ps, func(i, j int) bool {
		si, sj := math.Abs(ps[i].x2-ps[i].x1), math.Abs(ps[j].x2-ps[j].x1)
		if si != sj {
			return si < sj
		}
		return ps[i].lo < ps[j].lo
	})
	return ps
}

// assignLevels gives each arc the lowest level on its side of the row with room
// for it. It returns the number of levels above the row, at key true, and below
// it.
func assignLevels(ps []*placedArc) map[bool]int {
	levels := map[bool][][][2]float64{}
	maxLevel := map[bool]int{}
	for _, p := range ps {
		l := 0
		for ; ; l++ {
			for len(levels[p.above]) <= l {
				levels[p.above] = append(levels[p.above], nil)
			}
			if levelFree(levels[p.above][l], p) {
				break
			}
		}
		levels[p.above][l] = append(levels[p.above][l], [2]float64{p.lo, p.hi})
		p.level = l
		if l+1 > maxLevel[p.above] {
			maxLevel[p.above] = l + 1
		}
	}
	return maxLevel
}

// levelFree reports whether p stays clear of the arcs of a level, each given by
// its bounds.
func levelFree(level [][2]float64, p *placedArc) bool {
	for _, iv := range level {
		if p.lo < iv[1]+6 && iv[0] < p.hi+6 {
			return false
		}
	}
	return true
}

func arcHeight(level int) float64 {
	return baseH + float64(level)*levelH
}

// arcSpace returns the height of the space for the given number of levels of
// arcs.
func arcSpace(levels int) float64 {
	space := pad
	if levels > 0 {
		space += arcHeight(levels-1)*0.75 + 22
	}
	return space
}

// writeArc writes an arc and its label for a row of boxes at rowY.
func writeArc(b *strings.Builder, p *placedArc, rowY float64) {
	hh := arcHeight(p.level)
	y := rowY
	dir := -1.0
	color, mk := "#2b5fa8", "f"
	if !p.above {
		y = rowY + boxH
		dir = 1
		color, mk = "#a0562a", "b"
	}
	cy := y + dir*hh
	fmt.Fprintf(b, `<path d="M%.1f %.1fC%.1f %.1f %.1f %.1f %.1f %.1f" fill="none" stroke="%s" stroke-width="1.5" marker-end="url(#%s)"/>`+"\n", p.x1, y, p.x1, cy, p.x2, cy, p.x2, y, color, mk)
	ly := y + dir*hh*0.75
	if p.above {
		ly -= 5
	} else {
		ly += 14
	}
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="%d" text-anchor="middle" fill="%s" stroke="#ffffff" stroke-width="4" paint-order="stroke">%s</text>`+"\n", p.lx, ly, labelPx, color, html.EscapeString(p.label))
}
