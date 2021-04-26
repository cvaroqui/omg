package object

import "opensvc.com/opensvc/util/capabilities"

// OptsNodeScanCapabilities is the options of the NodeScanCapabilities function.
type OptsNodeScanCapabilities struct {
	Global OptsGlobal
}

// NodeScanCapabilities scan node capabilities and update node capability file
func (t Node) NodeScanCapabilities() (interface{}, error) {
	return nil, capabilities.Scan()
}
