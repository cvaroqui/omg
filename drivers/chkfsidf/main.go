package chkfsidf

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/core/check/helpers/checkdf"
	"opensvc.com/opensvc/util/df"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "fs_i"
	// DriverName is the name of check driver.
	DriverName = "df"
)

type fsChecker struct{}

func init() {
	check.Register(&fsChecker{})
}

func (t *fsChecker) Entries() ([]df.Entry, error) {
	return df.Inode()
}

// ObjectPath returns the path of the first object using the mount point
// passed as argument
func (t *fsChecker) objectPath(_ string) string {
	return ""
}

func (t *fsChecker) ResultSet(entry *df.Entry) *check.ResultSet {
	path := t.objectPath(entry.MountPoint)
	rs := check.NewResultSet()
	rs.Push(check.Result{
		Instance:    entry.MountPoint,
		Value:       entry.UsedPercent,
		Path:        path,
		Unit:        "%",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	rs.Push(check.Result{
		Instance:    entry.MountPoint + ".free",
		Value:       entry.Free,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	rs.Push(check.Result{
		Instance:    entry.MountPoint + ".size",
		Value:       entry.Total,
		Path:        path,
		Unit:        "inode",
		DriverGroup: DriverGroup,
		DriverName:  DriverName,
	})
	return rs
}

func (t *fsChecker) Check() (*check.ResultSet, error) {
	return checkdf.Check(t)
}

func main() {
	checker := &fsChecker{}
	if err := check.Check(checker); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
