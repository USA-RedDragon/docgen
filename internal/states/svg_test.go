package states

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type svgRect struct {
	X           float64 `xml:"x,attr"`
	Y           float64 `xml:"y,attr"`
	Width       float64 `xml:"width,attr"`
	Height      float64 `xml:"height,attr"`
	Stroke      string  `xml:"stroke,attr"`
	StrokeWidth string  `xml:"stroke-width,attr"`
}

type svgPath struct {
	D         string `xml:"d,attr"`
	Stroke    string `xml:"stroke,attr"`
	MarkerEnd string `xml:"marker-end,attr"`
}

type svgText struct {
	X    float64 `xml:"x,attr"`
	Y    float64 `xml:"y,attr"`
	Body string  `xml:",chardata"`
}

type svgDoc struct {
	Width   float64   `xml:"width,attr"`
	Height  float64   `xml:"height,attr"`
	ViewBox string    `xml:"viewBox,attr"`
	Rects   []svgRect `xml:"rect"`
	Paths   []svgPath `xml:"path"`
	Texts   []svgText `xml:"text"`
}

func parseSVG(t *testing.T, b []byte) svgDoc {
	t.Helper()
	var d svgDoc
	if err := xml.Unmarshal(b, &d); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, b)
	}
	return d
}

// boxes returns the state boxes, without the background rectangle.
func (d svgDoc) boxes() []svgRect {
	var out []svgRect
	for _, r := range d.Rects {
		if r.Stroke != "" {
			out = append(out, r)
		}
	}
	return out
}

func (d svgDoc) text(t *testing.T, body string) svgText {
	t.Helper()
	for _, x := range d.Texts {
		if x.Body == body {
			return x
		}
	}
	t.Fatalf("no text %q in %+v", body, d.Texts)
	return svgText{}
}

// controlY returns the y of the start and of the control points of an arc.
func controlY(t *testing.T, p svgPath) (start, control float64) {
	t.Helper()
	var x1, y1, c1x, c1y, c2x, c2y, x2, y2 float64
	if _, err := fmt.Sscanf(p.D, "M%g %gC%g %g %g %g %g %g", &x1, &y1, &c1x, &c1y, &c2x, &c2y, &x2, &y2); err != nil {
		t.Fatalf("path %q: %v", p.D, err)
	}
	return y1, c1y
}

func TestStateSVGBoxes(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("LongStateName", 3)
	d := parseSVG(t, stateSVG([]string{"Off", long, "On"}, nil, map[string]bool{"2": true}, []string{"0", "1", "2"}))
	boxes := d.boxes()
	if len(boxes) != 3 {
		t.Fatalf("%d boxes, want 3", len(boxes))
	}
	if !sort.SliceIsSorted(boxes, func(i, j int) bool { return boxes[i].X < boxes[j].X }) {
		t.Errorf("boxes not in state order: %+v", boxes)
	}
	if boxes[0].Width != 96 || boxes[1].Width <= 96 {
		t.Errorf("widths %g and %g: want the minimum and a wider box", boxes[0].Width, boxes[1].Width)
	}
	for i, want := range []string{"1.5", "1.5", "3"} {
		if boxes[i].StrokeWidth != want {
			t.Errorf("box %d stroke %s, want %s", i, boxes[i].StrokeWidth, want)
		}
	}
	if len(d.Paths) != 0 {
		t.Errorf("arcs with no transition: %+v", d.Paths)
	}
	for i, b := range boxes[1:] {
		if prev := boxes[i]; b.X <= prev.X+prev.Width {
			t.Errorf("boxes %d and %d overlap", i, i+1)
		}
	}
}

func TestStateSVGArcs(t *testing.T) {
	t.Parallel()
	arcs := []arc{{0, 1, "Start"}, {1, 0, "Stop"}, {1, 1, "Stay"}}
	d := parseSVG(t, stateSVG([]string{"Off", "On"}, arcs, nil, []string{"0", "1"}))
	if len(d.Paths) != 2 {
		t.Fatalf("%d arcs, want 2: a transition to the same state has no arc", len(d.Paths))
	}
	row := d.boxes()[0]
	fwd, back := d.Paths[0], d.Paths[1]
	if fwd.MarkerEnd != "url(#f)" || back.MarkerEnd != "url(#b)" || fwd.Stroke == back.Stroke {
		t.Errorf("forward %+v, backward %+v", fwd, back)
	}
	if y, c := controlY(t, fwd); y != row.Y || c >= y {
		t.Errorf("forward arc: start %g, control %g, row top %g", y, c, row.Y)
	}
	if y, c := controlY(t, back); y != row.Y+row.Height || c <= y {
		t.Errorf("backward arc: start %g, control %g, row bottom %g", y, c, row.Y+row.Height)
	}
	if d.text(t, "Start").Y >= row.Y || d.text(t, "Stop").Y <= row.Y+row.Height {
		t.Errorf("labels at %g and %g, row %g to %g", d.text(t, "Start").Y, d.text(t, "Stop").Y, row.Y, row.Y+row.Height)
	}
	for _, x := range d.Texts {
		if x.Body == "Stay" {
			t.Error("a label for a transition to the same state")
		}
	}
	if d.Height <= row.Y+row.Height {
		t.Errorf("height %g does not hold the row", d.Height)
	}
}

func TestStateSVGLevels(t *testing.T) {
	t.Parallel()
	nodes := []string{"A", "B", "C", "D"}
	keys := []string{"0", "1", "2", "3"}
	apart := parseSVG(t, stateSVG(nodes, []arc{{0, 1, "x"}, {2, 3, "y"}}, nil, keys))
	_, c0 := controlY(t, apart.Paths[0])
	_, c1 := controlY(t, apart.Paths[1])
	if c0 != c1 {
		t.Errorf("arcs that do not overlap have heights %g and %g", c0, c1)
	}
	over := parseSVG(t, stateSVG(nodes, []arc{{0, 3, "long"}, {1, 2, "short"}}, nil, keys))
	heights := make([]float64, 0, len(over.Paths))
	for _, p := range over.Paths {
		y, c := controlY(t, p)
		heights = append(heights, y-c)
	}
	if len(heights) != 2 || heights[0] == heights[1] {
		t.Errorf("arcs that overlap have heights %v", heights)
	}
	if over.Height <= apart.Height {
		t.Errorf("height %g with two levels, %g with one", over.Height, apart.Height)
	}
}

func TestStateSVGLabelInView(t *testing.T) {
	t.Parallel()
	label := strings.Repeat("VeryLongMethodName, ", 8)
	d := parseSVG(t, stateSVG([]string{"A", "B"}, []arc{{0, 1, label}}, nil, []string{"0", "1"}))
	f := strings.Fields(d.ViewBox)
	if len(f) != 4 {
		t.Fatalf("viewBox %q", d.ViewBox)
	}
	minX, _ := strconv.ParseFloat(f[0], 64)
	width, _ := strconv.ParseFloat(f[2], 64)
	if minX >= 0 {
		t.Errorf("viewBox starts at %g, want room to the left of the first box", minX)
	}
	if width != d.Width {
		t.Errorf("viewBox width %g, svg width %g", width, d.Width)
	}
	half := float64(len(label)) * charW * labelPx / fontPx / 2
	if x := d.text(t, label).X; x-half < minX || x+half > minX+width {
		t.Errorf("label from %g to %g, view from %g to %g", x-half, x+half, minX, minX+width)
	}
}

func TestStateSVGEscapes(t *testing.T) {
	t.Parallel()
	b := stateSVG([]string{`a<b&"c"`, "d"}, []arc{{0, 1, "x<y&z"}}, nil, []string{"0", "1"})
	if bytes.Contains(b, []byte("a<b")) || bytes.Contains(b, []byte("x<y")) {
		t.Errorf("text is not escaped:\n%s", b)
	}
	d := parseSVG(t, b)
	d.text(t, `a<b&"c"`)
	d.text(t, "x<y&z")
}

func TestStateSVGStable(t *testing.T) {
	t.Parallel()
	nodes := []string{"A", "B", "C"}
	arcs := []arc{{0, 2, "a"}, {0, 1, "b"}, {2, 0, "c"}, {1, 2, "d"}}
	first := stateSVG(nodes, arcs, map[string]bool{"0": true}, []string{"0", "1", "2"})
	for range 20 {
		if got := stateSVG(nodes, arcs, map[string]bool{"0": true}, []string{"0", "1", "2"}); !bytes.Equal(got, first) {
			t.Fatalf("output differs:\n%s\n%s", first, got)
		}
	}
	if !bytes.HasPrefix(first, []byte("<svg ")) || !bytes.HasSuffix(first, []byte("</svg>\n")) {
		t.Errorf("frame of the output:\n%s", first)
	}
}

// methodsOf returns "name:receiver" for each func of src, sorted by name.
func methodsOf(t *testing.T, src string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			name, _ := recvName(fd)
			out = append(out, fd.Name.Name+":"+name)
		}
	}
	sort.Strings(out)
	return out
}
