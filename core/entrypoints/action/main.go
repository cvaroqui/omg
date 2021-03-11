package action

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"opensvc.com/opensvc/core/entrypoints/monitor"
)

type (
	// Action switches between local, remote or async mode for a command action
	Action struct {
		//
		// ObjectSelector expands into a selection of objects to execute
		// the action on.
		//
		ObjectSelector string

		//
		// NodeSelector expands into a selection of nodes to execute the
		// action on.
		//
		NodeSelector string

		//
		// Local routes the action to the CRM instead of remoting it via
		// orchestration or remote execution.
		//
		Local bool

		//
		// DefaultIsLocal makes actions not explicitely Local nor remoted
		// via NodeSelector be treated as local (CRM level).
		//
		DefaultIsLocal bool

		//
		// Action is the name of the action as passed to the command line
		// interface.
		//
		Action string

		//
		// PostFlags is the dataset submited in the POST /{object|node}_action
		// api handler to execute the action remotely.
		//
		PostFlags map[string]interface{}

		//
		// Flags is the command flags as parsed by cobra. This is the struct
		// passed to the object method on local execution.
		//
		Flags interface{}

		//
		// Method is the func name called by the local execution, in the object
		// structure. Example "Start" call Svc{}.Start(...)
		//
		Method string

		//
		// MethodArgs is the list of arguments passed to the Method.
		//
		MethodArgs []interface{}

		//
		// Target is the node or object state the daemons should orchestrate
		// to reach.
		//
		Target string

		//
		// Watch runs a event-driven monitor on the selected objects after
		// setting a new target. So the operator can see the orchestration
		// unfolding.
		//
		Watch bool

		//
		// Format controls the output data format.
		// <empty>   => human readable format
		// json      => json machine readable format
		// flat      => flattened json (<k>=<v>) machine readable format
		// flat_json => same as flat (backward compat)
		//
		Format string

		//
		// Color activates the colorization of outputs
		// auto => yes if os.Stdout is a tty
		// yes
		// no
		//
		Color string

		//
		// Server bypasses the agent api requester automatic selection. It
		// Accepts a uri where the scheme can be:
		// raw   => jsonrpc
		// http  => http/2 cleartext (over unix domain socket only)
		// https => http/2 with TLS
		// tls   => http/2 with TLS
		//
		Server string

		// Lock prevents the action to run concurrently on the node.
		Lock bool

		// LockTimeout decides how long to wait for the lock before returning
		// an error.
		LockTimeout time.Duration

		// LockGroup specifies an alternate lockfile, so this action is can not
		// be run in parallel with the action of the same group, but can run in
		// parallel with the actions of the default group.
		LockGroup string
	}

	// actioner is a interface implemented for node and object.
	actioner interface {
		doRemote()
		doLocal()
		doAsync()
		options() Action
	}
)

// Do is the switch method between local, remote or async mode.
// If Watch is set, end up starting a monitor on the selected objects.
func Do(t actioner) {
	o := t.options()
	switch {
	case o.NodeSelector != "":
		t.doRemote()
	case o.Local || o.DefaultIsLocal:
		t.doLocal()
	case o.Target != "":
		t.doAsync()
	default:
		log.Errorf("no available method to run action %s", t)
		os.Exit(1)
	}
	if o.Watch {
		m := monitor.New()
		m.SetWatch(true)
		m.SetColor(o.Color)
		m.SetFormat(o.Format)
		m.SetSelector(o.ObjectSelector)
		m.SetServer(o.Server)
		m.Do()
	}
}