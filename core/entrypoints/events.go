package entrypoints

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

// Events hosts the options of the events fetcher/renderer entrypoint.
type Events struct {
	Color  string
	Format string
	Server string
}

// Do renders the event stream
func (t Events) Do() {
	var (
		err error
		c   *client.T
	)
	c, err = client.New(client.WithURL(t.Server))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	streamer := c.NewGetEvents().SetRelatives(false)
	events, err := streamer.Do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for m := range events {
		t.doOne(m)
	}
}

func (t Events) doOne(e event.Event) {
	human := func() string {
		return event.Render(e)
	}
	if t.Format == output.JSON.String() {
		t.Format = output.JSONLine.String()
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          e,
		HumanRenderer: human,
		Colorize:      rawconfig.Node.Colorize,
	}.Print()
}
