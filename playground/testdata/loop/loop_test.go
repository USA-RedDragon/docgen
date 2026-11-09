package loop

import "testing"

var n int

func TestForever(t *testing.T) {
	for {
		n++
	}
}
