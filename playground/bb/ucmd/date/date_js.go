//go:build js

package udate

import (
	"errors"
	"time"
)

func setDate(d string, z *time.Location, clocksource Clock) error {
	return errors.New("setting the date is not supported on js/wasm")
}
