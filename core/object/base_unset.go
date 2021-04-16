package object

import (
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsUnset struct {
	Global   OptsGlobal
	Lock     OptsLocking
	Keywords []string `flag:"kws"`
}

// Unset gets a keyword value
func (t *Base) Unset(options OptsUnset) error {
	changes := 0
	for _, kw := range options.Keywords {
		k := key.Parse(kw)
		changes += t.config.Unset(k)
	}
	if changes > 0 {
		return t.config.Commit()
	}
	return nil
}
