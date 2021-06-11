package findmnt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"opensvc.com/opensvc/util/file"
)

type (
	MountInfo struct {
		Source  string `json:"source"`
		Target  string `json:"target"`
		FsType  string `json:"fstype"`
		Options string `json:"options"`
	}
	info struct {
		Filesystems []MountInfo `json:"filesystems"`
	}
)

func Has(dev string, mnt string) (bool, error) {
	l, err := List(dev, mnt)
	if err != nil {
		return false, err
	}
	return len(l) > 0, nil
}

func newInfo() *info {
	data := info{}
	data.Filesystems = make([]MountInfo, 0)
	return &data
}

func List(dev string, mnt string) ([]MountInfo, error) {
	data := newInfo()
	if _, err := exec.LookPath("findmnt"); err != nil {
		return data.Filesystems, err
	}
	bind := file.ExistsAndDir(dev)
	opts := []string{"-J", "-T", mnt}
	if dev != "" && !bind {
		opts = append(opts, "-S", dev)
	}
	cmd := exec.Command("findmnt", opts...)
	stdout, err := cmd.Output()
	if err != nil {
		return data.Filesystems, nil
	}
	err = json.Unmarshal(stdout, &data)
	if err != nil {
		return data.Filesystems, err
	}
	if bind {
		filtered := newInfo()
		pattern := fmt.Sprintf("[%s]", mnt)
		for _, mi := range data.Filesystems {
			if !strings.Contains(mi.Source, pattern) {
				continue
			}
			filtered.Filesystems = append(filtered.Filesystems, mi)
		}
	}
	return data.Filesystems, nil
}
