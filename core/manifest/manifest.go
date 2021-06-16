package manifest

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

type (
	//
	// T describes a driver so callers can format the input as the
	// driver expects.
	//
	// A typical allocation is:
	// m := New("fs", "flag").AddKeyword(kws...).AddContext(ctx...)
	//
	T struct {
		Group    drivergroup.T      `json:"group"`
		Name     string             `json:"name"`
		Keywords []keywords.Keyword `json:"keywords"`
		Context  []Context          `json:"context"`
	}

	//
	// Context is a key-value the resource expects to find in the input,
	// merged with keywords coming from configuration file.
	//
	// For example, a driver often needs the parent object Path, which
	// can be asked via:
	//
	// T{
	//     Context: []Context{
	//         {
	//             Key: "path",
	//             Ref:"object.path",
	//         },
	//     },
	// }
	//
	Context struct {
		// Key is the name of the key in the json representation of the context.
		Key string

		// Attr is the name of the field in the resource struct.
		Attr string

		// Ref is the code describing what context information to embed in the resource struct.
		Ref string
	}
)

var genericKeywords = []keywords.Keyword{
	{
		Option:    "disable",
		Attr:      "Disable",
		Converter: converters.Bool,
		Text:      "A disabled resource will be ignored on service startup and shutdown. Its status is always reported ``n/a``.\n\nSet in DEFAULT, the whole service is disabled. A disabled service does not honor :c-action:`start` and :c-action:`stop` actions. These actions immediately return success.\n\n:cmd:`om <path> disable` only sets :kw:`DEFAULT.disable`. As resources disabled state is not changed, :cmd:`om <path> enable` does not enable disabled resources.",
	},
	{
		Option:    "optional",
		Attr:      "Optional",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Action failures on optional resources are logged but do not stop the action sequence. Also the optional resource status is not aggregated to the instance 'availstatus', but aggregated to the 'overallstatus'. Resource tagged :c-tag:`noaction` and sync resources are automatically considered optional. Useful for resources like dump filesystems for example.",
	},
	{
		Option:    "shared",
		Attr:      "Shared",
		Converter: converters.Bool,
		Text:      "Set to ``true`` to skip the resource on provision and unprovision actions if the action has already been done by a peer. Shared resources, like vg built on SAN disks must be provisioned once. All resources depending on a shared resource must also be flagged as shared.",
	},
	{
		Option:    "standby",
		Attr:      "Standby",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "Always start the resource, even on standby instances. The daemon is responsible for starting standby resources. A resource can be set standby on a subset of nodes using keyword scoping.\n\nA typical use-case is sync'ed fs on non-shared disks: the remote fs must be mounted to not overflow the underlying fs.\n\n.. warning:: Don't set shared resources standby: fs on shared disks for example.",
	},
	{
		Option:    "tags",
		Attr:      "Tags",
		Scopable:  true,
		Converter: converters.Set,
		Text:      "A list of tags. Arbitrary tags can be used to limit action scope to resources with a specific tag. Some tags can influence the driver behaviour. For example :c-tag:`noaction` avoids any state changing action from the driver and implies ``optional=true``, :c-tag:`nostatus` forces the status to n/a.",
	},
	{
		Option:   "subset",
		Attr:     "Subset",
		Scopable: true,
		Text:     "Assign the resource to a specific subset.",
	},
	{
		Option:   "blocking_pre_start",
		Attr:     "BlockingPreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_pre_stop",
		Attr:     "BlockingPreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors interrupt the action.",
	},
	{
		Option:   "pre_start",
		Attr:     "PreStart",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`start` action. Errors do not interrupt the action.",
	},
	{
		Option:   "pre_stop",
		Attr:     "PreStop",
		Scopable: true,
		Text:     "A command or script to execute before the resource :c-action:`stop` action. Errors do not interrupt the action.",
	},
	{
		Option:   "blocking_post_start",
		Attr:     "BlockingPostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors interrupt the action.",
	},
	{
		Option:   "blocking_post_stop",
		Attr:     "BlockingPostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors interrupt the action.",
	},
	{
		Option:   "post_start",
		Attr:     "PostStart",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`start` action. Errors do not interrupt the action.",
	},
	{
		Option:   "post_stop",
		Attr:     "PostStop",
		Scopable: true,
		Text:     "A command or script to execute after the resource :c-action:`stop` action. Errors do not interrupt the action.",
	},
}

func New(group drivergroup.T, name string) *T {
	t := &T{
		Group: group,
		Name:  name,
	}
	t.Keywords = append(t.Keywords, genericKeywords...)
	return t
}

func (t *T) AddKeyword(kws ...keywords.Keyword) *T {
	t.Keywords = append(t.Keywords, kws...)
	return t
}

func (t *T) AddContext(ctx ...Context) *T {
	t.Context = append(t.Context, ctx...)
	return t
}
