package object

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/util/key"
)

const (
	DefaultInstalledFileMode os.FileMode = 0644
)

// OptsDecode is the options of the Decode function of all keystore objects.
type OptsDecode struct {
	Global OptsGlobal
	Lock   OptsLocking
	Key    string `flag:"key"`
}

// Get returns a keyword value
func (t *Keystore) decode(keyname string, cd CustomDecoder) ([]byte, error) {
	var (
		s   string
		err error
	)
	if keyname == "" {
		return []byte{}, fmt.Errorf("key name can not be empty")
	}
	k := key.New(DataSectionName, keyname)
	if s, err = t.config.GetStringStrict(k); err != nil {
		return []byte{}, err
	}
	return cd.CustomDecode(s)
}