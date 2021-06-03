package converters

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"opensvc.com/opensvc/util/converters/sizeconv"

	"github.com/anmitsu/go-shlex"
	"github.com/golang-collections/collections/set"
)

type (
	TString        string
	TInt           string
	TInt64         string
	TFloat64       string
	TBool          string
	TList          string
	TListLowercase string
	TSet           string
	TShlex         string
	TDuration      string
	TUmask         string
	TSize          string
)

var (
	String        TString
	Int           TInt
	Int64         TInt64
	Float64       TFloat64
	Bool          TBool
	List          TList
	ListLowercase TListLowercase
	Set           TSet
	Shlex         TShlex
	Duration      TDuration
	Umask         TUmask
	Size          TSize
)

//
func (t TString) Convert(s string) (interface{}, error) {
	return s, nil
}

func (t TString) String() string {
	return "string"
}

//
func (t TInt) Convert(s string) (interface{}, error) {
	return strconv.Atoi(s)
}

func (t TInt) String() string {
	return "int"
}

//
func (t TInt64) Convert(s string) (interface{}, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (t TInt64) String() string {
	return "int64"
}

//
func (t TFloat64) Convert(s string) (interface{}, error) {
	return strconv.ParseFloat(s, 64)
}

func (t TFloat64) String() string {
	return "float64"
}

//
func (t TBool) Convert(s string) (interface{}, error) {
	if s == "" {
		return false, nil
	}
	return strconv.ParseBool(s)
}

func (t TBool) String() string {
	return "bool"
}

//
func (t TList) Convert(s string) (interface{}, error) {
	return strings.Fields(s), nil
}

func (t TList) String() string {
	return "list"
}

//
func (t TListLowercase) Convert(s string) (interface{}, error) {
	l := strings.Fields(s)
	for i := 0; i < len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func (t TListLowercase) String() string {
	return "list-lowercase"
}

//
func (t TSet) Convert(s string) (interface{}, error) {
	aSet := set.New()
	for _, e := range strings.Fields(s) {
		aSet.Insert(e)
	}
	return aSet, nil
}

func (t TSet) String() string {
	return "set"
}

//
func (t TShlex) Convert(s string) (interface{}, error) {
	return shlex.Split(s, true)
}

func (t TShlex) String() string {
	return "shlex"
}

//
// ToDuration convert duration string to *time.Duration
//
// nil is returned when duration is unset
// Default unit is second when not specified
//
func (t TDuration) Convert(s string) (interface{}, error) {
	return t.convert(s)
}

func (t TDuration) convert(s string) (*time.Duration, error) {
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		s = s + "s"
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}
	return &duration, nil
}

func (t TDuration) String() string {
	return "duration"
}

//
func (t TUmask) Convert(s string) (interface{}, error) {
	return t.convert(s)
}

func (t TUmask) convert(s string) (*fs.FileMode, error) {
	if s == "" {
		return nil, nil
	}
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return nil, errors.New("unexpected umask value: " + s + " " + err.Error())
	}
	umask := os.FileMode(i)
	return &umask, nil
}

func (t TUmask) String() string {
	return "umask"
}

//
func (t TSize) Convert(s string) (interface{}, error) {
	return t.convert(s)
}

func (t TSize) convert(s string) (*int64, error) {
	var (
		err error
		i   int64
	)
	if s == "" {
		return nil, err
	}
	if i, err = sizeconv.FromSize(s); err != nil {
		return nil, err
	}
	return &i, err
}

func (t TSize) String() string {
	return "size"
}
