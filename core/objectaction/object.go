package objectaction

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	// T has the same attributes as Action, but the interface
	// method implementation differ.
	T struct {
		action.T
		Object object.Action
	}
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
	return t
}

//
// WithObjectSelector expands into a selection of objects to execute
// the action on.
//
func WithObjectSelector(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.ObjectSelector = s
		return nil
	})
}

//
// WithRemoteNodes expands into a selection of nodes to execute the
// action on.
//
func WithRemoteNodes(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.NodeSelector = s
		return nil
	})
}

//
// WithLocal routes the action to the CRM instead of remoting it via
// orchestration or remote execution.
//
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Local = v
		return nil
	})
}

//
// LocalFirst makes actions not explicitely Local nor remoted
// via NodeSelector be treated as local (CRM level).
//
func LocalFirst() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.DefaultIsLocal = true
		return nil
	})
}

//
// WithRemoteAction is the name of the action as passed to the command line
// interface.
//
func WithRemoteAction(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Action = s
		return nil
	})
}

//
// WithRemoteOptions is the dataset submited in the POST /{object|node}_action
// api handler to execute the action remotely.
//
func WithRemoteOptions(m map[string]interface{}) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.PostFlags = m
		return nil
	})
}

//
// WithAsyncTarget is the node or object state the daemons should orchestrate
// to reach.
//
func WithAsyncTarget(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Target = s
		return nil
	})
}

//
// WithAsyncWatch runs a event-driven monitor on the selected objects after
// setting a new target. So the operator can see the orchestration
// unfolding.
//
func WithAsyncWatch(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Watch = v
		return nil
	})
}

//
// WithFormat controls the output data format.
// <empty>   => human readable format
// json      => json machine readable format
// flat      => flattened json (<k>=<v>) machine readable format
// flat_json => same as flat (backward compat)
//
func WithFormat(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Format = s
		return nil
	})
}

//
// WithColor activates the colorization of outputs
// auto => yes if os.Stdout is a tty
// yes
// no
//
func WithColor(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Color = s
		return nil
	})
}

//
// WithServer sets the api url.
//
func WithServer(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Server = s
		return nil
	})
}

// WithLocalRun sets a function to run if the the action is local
func WithLocalRun(f func(path.T) (interface{}, error)) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Object.Run = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() action.T {
	return t.T
}

func (t T) DoLocal() error {
	log.Debug().
		Str("format", t.Format).
		Str("selector", t.ObjectSelector).
		Msg("do local object selection action")
	sel := object.NewSelection(
		t.ObjectSelector,
		object.SelectionWithLocal(true),
	)
	rs := sel.Do(t.Object)
	human := func() string {
		s := ""
		for _, r := range rs {
			switch {
			case errors.Is(r.Error, object.ErrLogged):
				// do not log again
			case r.Error != nil:
				log.Error().Err(r.Error).Msg("")
			case r.Panic != nil:
				switch err := r.Panic.(type) {
				case error:
					log.Fatal().Stack().Err(err).Msg("")
				default:
					log.Fatal().Msgf("%s", err)
				}
			case r.HumanRenderer != nil:
				s += r.HumanRenderer()
			case r.Data != nil:
				switch v := r.Data.(type) {
				case string:
					s += fmt.Sprintln(v)
				case []string:
					for _, e := range v {
						s += fmt.Sprintln(e)
					}
				default:
					log.Error().Msgf("unimplemented default renderer for local action result of type %s", reflect.TypeOf(v))
				}
			}
		}
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          rs,
		HumanRenderer: human,
		Colorize:      rawconfig.Node.Colorize,
	}.Print()
	for _, ar := range rs {
		if ar.Panic != nil || ar.Error != nil {
			return errors.New("")
		}
	}
	return nil
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	sel := object.NewSelection(
		t.ObjectSelector,
		object.SelectionWithClient(c),
	)
	for _, path := range sel.Expand() {
		req := c.NewPostObjectMonitor()
		req.ObjectSelector = path.String()
		req.GlobalExpect = t.Target
		req.SetNode(t.NodeSelector)
		b, err := req.Do()
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		human := func() string {
			s := fmt.Sprintln(string(b))
			return s
		}
		output.Renderer{
			Format:        t.Format,
			Color:         t.Color,
			Data:          b,
			HumanRenderer: human,
			Colorize:      rawconfig.Node.Colorize,
		}.Print()
	}
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	req := c.NewPostObjectAction()
	req.ObjectSelector = t.ObjectSelector
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action
	req.Options = t.PostFlags
	b, err := req.Do()
	human := func() string {
		s := fmt.Sprintln(string(b))
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          b,
		HumanRenderer: human,
	}.Print()
}

func (t T) Do() {
	err := action.Do(t)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
