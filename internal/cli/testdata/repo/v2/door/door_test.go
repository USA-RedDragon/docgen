package door

import "testing"

func TestOpen(t *testing.T) {
	var d Door
	d.Open()
}

func TestClose(t *testing.T) {
	var d Door
	d.Open()
	d.Close()
}
