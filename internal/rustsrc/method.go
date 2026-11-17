package rustsrc

// A Method is the signature and the body of a fn of an inherent impl.
type Method struct {
	// Receiver is "" for no self, ref for &self, mut for &mut self, value for
	// self or mut self, and typed for self: Type.
	Receiver string `json:"receiver"`
	// Params holds the parameters with an identifier pattern.
	Params   []Param `json:"params"`
	Async    bool    `json:"async"`
	Const    bool    `json:"const"`
	Generics bool    `json:"generics"`
	// BodyStart is the byte offset after the opening brace of the body and
	// its inner attributes, and BodyEnd the offset of its closing brace.
	BodyStart int `json:"body_start"`
	BodyEnd   int `json:"body_end"`
}

// A Param is a parameter of a Method.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// The receivers of a Method.
const (
	RecvNone  = ""
	RecvRef   = "ref"
	RecvMut   = "mut"
	RecvValue = "value"
	RecvTyped = "typed"
)
