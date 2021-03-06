package xconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"gopkg.in/ini.v1"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	Converter func(string) (interface{}, error)

	// T exposes methods to read and write configurations.
	T struct {
		ConfigFilePath string
		Path           path.T
		Referrer       Referrer
		NodeReferrer   Referrer
		file           *ini.File
	}

	// Referer is the interface implemented by node and object to
	// provide a reference resolver using their private attributes.
	Referrer interface {
		KeywordLookup(key.T, string) keywords.Keyword
		PostCommit() error
		IsVolatile() bool
		Log() *zerolog.Logger
		Config() *T

		// for reference private to the referrer. ex: path for an object
		Dereference(string) (string, error)

		// for scoping
		Nodes() []string
		DRPNodes() []string
		EncapNodes() []string
	}

	ErrPostponedRef struct {
		Ref string
		RID string
	}
)

var (
	RegexpOperation = regexp.MustCompile(`(\$\(\(.+\)\))`)
	ErrExist        = errors.New("configuration does not exist")
	ErrNoKeyword    = errors.New("keyword does not exist")

	DriverGroups = set.New("ip", "volume", "disk", "fs", "share", "container", "app", "sync", "task")
)

func (t ErrPostponedRef) Error() string {
	return fmt.Sprintf("ref %s evaluation postponed: resource %s is not configured yet", t.Ref, t.RID)
}

func NewErrPostponedRef(ref string, rid string) ErrPostponedRef {
	return ErrPostponedRef{
		Ref: ref,
		RID: rid,
	}
}

// Keys returns the key names available in a section
func (t *T) Keys(section string) []string {
	data := make([]string, 0)
	for _, s := range t.file.Section(section).Keys() {
		data = append(data, s.Name())
	}
	return data
}

// HasKey returns true if the k exists
func (t *T) HasKey(k key.T) bool {
	return t.file.Section(k.Section).HasKey(k.Option)
}

func (t *T) Get(k key.T) string {
	val := t.file.Section(k.Section).Key(k.Option).Value()
	return val
}

func (t *T) GetStrict(k key.T) (string, error) {
	s := t.file.Section(k.Section)
	if s.HasKey(k.Option) {
		return s.Key(k.Option).Value(), nil
	}
	return "", errors.Wrapf(ErrExist, "key '%s' not found (unscopable kw)", k)
}

func (t *T) GetString(k key.T) string {
	val, _ := t.GetStringStrict(k)
	return val
}

func (t *T) GetStringStrict(k key.T) (string, error) {
	if v, err := t.Eval(k); err != nil {
		return "", err
	} else {
		return v.(string), nil
	}
}

func (t *T) GetSlice(k key.T) []string {
	val, _ := t.GetSliceStrict(k)
	return val
}

func (t *T) GetSliceStrict(k key.T) ([]string, error) {
	if v, err := t.Eval(k); err != nil {
		return []string{}, err
	} else {
		return v.([]string), nil
	}
}

func (t *T) GetBool(k key.T) bool {
	val, _ := t.GetBoolStrict(k)
	return val
}

func (t *T) GetBoolStrict(k key.T) (bool, error) {
	if v, err := t.Eval(k); err != nil {
		return false, err
	} else {
		return v.(bool), nil
	}
}

func (t *T) GetDuration(k key.T) *time.Duration {
	val, _ := t.GetDurationStrict(k)
	return val
}

func (t *T) GetDurationStrict(k key.T) (*time.Duration, error) {
	if v, err := t.Eval(k); err != nil {
		return nil, err
	} else {
		return v.(*time.Duration), nil
	}
}

func (t *T) GetInt(k key.T) int {
	val, _ := t.GetIntStrict(k)
	return val
}

func (t *T) GetIntStrict(k key.T) (int, error) {
	if v, err := t.Eval(k); err != nil {
		return 0, err
	} else {
		return v.(int), nil
	}
}

func (t *T) GetSize(k key.T) *int64 {
	val, _ := t.GetSizeStrict(k)
	return val
}

func (t *T) GetSizeStrict(k key.T) (*int64, error) {
	if v, err := t.Eval(k); err != nil {
		var i int64
		return &i, err
	} else {
		return v.(*int64), nil
	}
}

// Unset deletes keys and returns the number of deleted keys
func (t *T) Unset(ks ...key.T) int {
	deleted := 0
	for _, k := range ks {
		if !t.file.Section(k.Section).HasKey(k.Option) {
			continue
		}
		t.file.Section(k.Section).DeleteKey(k.Option)
		deleted += 1
	}
	return deleted
}

func (t *T) Set(op keyop.T) error {
	if !DriverGroups.Has(op.Key.Section) {
		return t.set(op)
	}
	return t.DriverGroupSet(op)
}

func (t *T) DriverGroupSet(op keyop.T) error {
	prefix := op.Key.Section + "#"
	for _, section := range t.file.SectionStrings() {
		if !strings.HasPrefix(section, prefix) {
			continue
		}
		op.Key.Section = section
		if err := t.set(op); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) set(op keyop.T) error {
	t.Referrer.Log().Debug().Stringer("op", op).Msg("set")
	setSet := func(op keyop.T) error {
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(op.Value)
		return nil
	}
	setAppend := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option).Value()
		target := ""
		if current == "" {
			target = op.Value
		} else {
			target = fmt.Sprintf("%s %s", current, op.Value)
		}
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(target)
		return nil
	}
	setMerge := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		currentSet := set.New()
		for _, e := range current {
			currentSet.Insert(e)
		}
		if currentSet.Has(op.Value) {
			return nil
		}
		return setAppend(op)
	}

	setRemove := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		target := []string{}
		removed := 0
		for _, e := range current {
			if e == op.Value {
				removed += 1
				continue
			}
			target = append(target, e)
		}
		if removed == 0 {
			return nil
		}
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(strings.Join(target, " "))
		return nil
	}

	setToggle := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		hasValue := false
		for _, e := range current {
			if e == op.Value {
				hasValue = true
				break
			}
		}
		if hasValue {
			return setRemove(op)
		}
		return setMerge(op)
	}

	setInsert := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		target := []string{}
		target = append(target, current[:op.Index]...)
		target = append(target, op.Value)
		target = append(target, current[op.Index:]...)
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(strings.Join(target, " "))
		return nil
	}

	switch op.Op {
	case keyop.Set:
		return setSet(op)
	case keyop.Append:
		return setAppend(op)
	case keyop.Remove:
		return setRemove(op)
	case keyop.Merge:
		return setMerge(op)
	case keyop.Toggle:
		return setToggle(op)
	case keyop.Insert:
		return setInsert(op)
	}
	return fmt.Errorf("unsupported operator: %d", op.Op)
}

func (t *T) write() (err error) {
	var f *os.File
	ini.DefaultHeader = true
	dir := filepath.Dir(t.ConfigFilePath)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	base := filepath.Base(t.ConfigFilePath)
	if f, err = ioutil.TempFile(dir, "."+base+".*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)
	if err = t.file.SaveTo(fName); err != nil {
		return err
	}
	if _, err = t.file.WriteTo(f); err != nil {
		return err
	}
	return os.Rename(fName, t.ConfigFilePath)
}

func (t *T) Eval(k key.T) (interface{}, error) {
	return t.EvalAs(k, "")
}

//
// Get returns a key value,
// * contextualized for a node (by default the local node, customized by the
//   impersonate option)
// * dereferenced
// * evaluated
//
func (t *T) EvalAs(k key.T, impersonate string) (interface{}, error) {
	sectionType := t.sectionType(k)
	kw, err := getKeyword(k, sectionType, t.Referrer)
	if err != nil {
		return nil, err
	}
	return t.EvalKeywordAs(k, kw, impersonate)
}

func (t *T) sectionType(k key.T) string {
	if k.Option == "type" {
		return ""
	}
	return t.GetString(key.New(k.Section, "type"))
}

func (t *T) EvalKeywordAs(k key.T, kw keywords.Keyword, impersonate string) (interface{}, error) {
	v, err := t.evalStringAs(k, kw, impersonate)
	if err != nil {
		return nil, err
	}
	return t.convert(v, kw)
}

func getKeyword(k key.T, sectionType string, referrer Referrer) (keywords.Keyword, error) {
	var kw keywords.Keyword
	if referrer == nil {
		return kw, errors.Wrapf(ErrNoKeyword, "no referrer")
	}
	kw = referrer.KeywordLookup(k, sectionType)
	if kw.IsZero() {
		return kw, errors.Wrapf(ErrNoKeyword, "%s", k)
	}
	return kw, nil
}

func (t *T) evalStringAs(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	v, err := t.mayDescope(k, kw, impersonate)
	if err != nil {
		return "", err
	}
	return t.replaceReferences(v, k.Section, impersonate)
}

func (t *T) convert(v string, kw keywords.Keyword) (interface{}, error) {
	if kw.Converter == nil {
		return v, nil
	}
	return kw.Converter.Convert(v)
}

func (t *T) mayDescope(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	var (
		v   string
		err error
	)
	if kw.Scopable {
		v, err = t.descope(k, impersonate)
	} else {
		v, err = t.GetStrict(k)
	}
	switch {
	case errors.Is(err, ErrExist):
		switch kw.Required {
		case true:
			return "", err
		case false:
			return kw.Default, nil
		}
	case err != nil:
		return "", err
	}
	return v, nil
}

func (t *T) replaceReferences(v string, section string, impersonate string) (string, error) {
	errs := make([]error, 0)
	v = rawconfig.RegexpReference.ReplaceAllStringFunc(v, func(ref string) string {
		var (
			s string
			e error
		)
		s, e = t.dereference(ref, section, impersonate)
		if e != nil {
			switch e.(type) {
			case ErrPostponedRef:
				errs = append(errs, e)
			}
			return ref
		}
		return s
	})
	for _, e := range errs {
		return v, e
	}
	return v, nil
}

func (t T) sectionMap(section string) (map[string]string, error) {
	s, err := t.file.GetSection(section)
	if err != nil {
		return nil, errors.Wrapf(ErrExist, "section '%s'", section)
	}
	return s.KeysHash(), nil
}

func (t *T) descope(k key.T, impersonate string) (string, error) {
	if impersonate == "" {
		impersonate = hostname.Hostname()
	}
	s, err := t.sectionMap(k.Section)
	if err != nil {
		return "", err
	}
	if v, ok := s[k.Option+"@"+impersonate]; ok {
		return v, nil
	}
	if v, ok := s[k.Option+"@nodes"]; ok && t.IsInNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option+"@drpnodes"]; ok && t.IsInDRPNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option+"@encapnodes"]; ok && t.IsInEncapNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option]; ok {
		return v, nil
	}
	return "", errors.Wrapf(ErrExist, "key '%s' not found (all scopes tried)", k)
}

func (t T) Raw() rawconfig.T {
	r := rawconfig.T{}
	r.Data = orderedmap.New()
	for _, s := range t.file.Sections() {
		sectionMap := *orderedmap.New()
		for k, v := range s.KeysHash() {
			sectionMap.Set(k, v)
		}
		r.Data.Set(s.Name(), sectionMap)
	}
	return r
}

func (t T) HasSectionString(s string) bool {
	for _, e := range t.SectionStrings() {
		if s == e {
			return true
		}
	}
	return false
}

func (t T) SectionStrings() []string {
	return t.file.SectionStrings()
}

func (t *T) IsInNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.Referrer.Nodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t *T) IsInDRPNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.Referrer.DRPNodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t *T) IsInEncapNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.Referrer.EncapNodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t T) dereference(ref string, section string, impersonate string) (string, error) {
	type f func(string) string
	var (
		modifier f
		err      error
	)
	val := ""
	ref = ref[1 : len(ref)-1]
	l := strings.SplitN(ref, ":", 2)
	switch l[0] {
	case "upper":
		modifier = strings.ToUpper
	case "lower":
		modifier = strings.ToLower
	case "capitalize":
		modifier = xstrings.Capitalize
	case "title":
		modifier = strings.Title
	case "swapcase":
		modifier = xstrings.SwapCase
	default:
		modifier = func(s string) string { return s }
	}
	switch {
	case strings.HasPrefix(ref, "node."):
		if val, err = t.dereferenceNodeKey(ref, impersonate); err != nil {
			return ref, err
		}
	default:
		if val, err = t.dereferenceWellKnown(ref, section, impersonate); err != nil {
			return ref, err
		}
	}
	return modifier(val), nil
}

func (t T) dereferenceNodeKey(ref string, impersonate string) (string, error) {
	t.Referrer.Log().Debug().Msgf("dereference node key %s", ref)

	//
	// Extract the key string relative to the node configuration
	// Examples:
	//   "node.env" => "env"
	//   "node.labels.az" => "labels.az"
	//
	l := strings.SplitN(ref, ".", 2)
	nodeRef := l[1]

	// Use "node" as the implicit section instead of "DEFAULT"
	if !strings.Contains(nodeRef, ".") {
		nodeRef = "node." + nodeRef
	}

	nodeKey := key.Parse(nodeRef)
	sectionType := t.sectionType(nodeKey)
	kw, err := getKeyword(nodeKey, sectionType, t.NodeReferrer)
	if err != nil {
		return ref, err
	}

	// Filter on node key section
	switch nodeKey.Section {
	case "env", "labels", "node":
		// allow
	default:
		// deny
		t.Referrer.Log().Debug().Msgf("denied reference to node key %s", ref)
		return ref, fmt.Errorf("denied reference to node key %s", ref)
	}

	val, err := t.NodeReferrer.Config().evalStringAs(nodeKey, kw, impersonate)
	if err != nil {
		return ref, err
	}
	return val, nil
}

func (t T) dereferenceKey(ref string, section string, impersonate string) (string, error) {
	t.Referrer.Log().Debug().Msgf("dereference well known key %s", ref)
	refKey := key.Parse(ref)
	if refKey.Section == "" {
		refKey.Section = section
	}
	key, err := t.file.Section(refKey.Section).GetKey(refKey.Option)
	if err != nil {
		return "", err
	}
	return t.replaceReferences(key.String(), refKey.Section, impersonate)
}

func (t T) dereferenceWellKnown(ref string, section string, impersonate string) (string, error) {
	if v, err := t.dereferenceKey(ref, section, impersonate); err == nil {
		return v, nil
	}
	switch ref {
	case "nodename":
		return impersonate, nil
	case "short_nodename":
		return strings.SplitN(impersonate, ".", 1)[0], nil
	case "rid":
		return section, nil
	case "rindex":
		l := strings.SplitN(section, "#", 2)
		if len(l) != 2 {
			return section, nil
		}
		return l[1], nil
	case "svcmgr":
		return os.Args[0] + " svc", nil
	case "nodemgr":
		return os.Args[0] + " node", nil
	case "etc":
		return rawconfig.Node.Paths.Etc, nil
	case "var":
		return rawconfig.Node.Paths.Var, nil
	default:
		if t.Referrer != nil {
			return t.Referrer.Dereference(ref)
		}
	}
	return ref, fmt.Errorf("unknown reference: %s", ref)
}

func (t *T) replaceFile(configData rawconfig.T) error {
	file := ini.Empty()
	for _, section := range configData.Data.Keys() {
		m, _ := configData.Data.Get(section)
		omap, ok := m.(orderedmap.OrderedMap)
		if !ok {
			return fmt.Errorf("invalid section in raw config format: %+v", m)
		}
		for _, option := range omap.Keys() {
			value, _ := omap.Get(option)
			var v string
			if value == nil {
				v = ""
			} else {
				v = value.(string)
			}
			file.Section(section).Key(option).SetValue(v)
		}
	}
	t.file = file
	return nil
}

func (t T) deleteSection(section string) {
	if _, err := t.file.GetSection(section); err != nil {
		return
	}
	t.file.DeleteSection(section)
}

func (t T) initDefaultSection() error {
	defaultSection, err := t.file.GetSection("DEFAULT")
	if err != nil {
		defaultSection, err = t.file.NewSection("DEFAULT")
		if err != nil {
			return err
		}
	}
	if !defaultSection.HasKey("id") {
		_, err = defaultSection.NewKey("id", uuid.New().String())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *T) rawCommit(configData rawconfig.T, configPath string, validate bool) error {
	if !configData.IsZero() {
		if err := t.replaceFile(configData); err != nil {
			return err
		}
	}
	if configPath == "" {
		configPath = t.ConfigFilePath
	}
	if len(t.file.Sections()) == 0 {
		return nil
	}
	t.deleteSection("metadata")
	if err := t.initDefaultSection(); err != nil {
		return err
	}
	if validate {
		if err := t.validate(); err != nil {
			return err
		}
	}
	if !t.Referrer.IsVolatile() {
		if err := t.write(); err != nil {
			return err
		}
	}
	//t.clearRefCache()
	return t.postCommit()
}

func (t T) validate() error {
	return nil
}

func (t *T) Commit() error {
	return t.rawCommit(rawconfig.T{}, "", true)
}

func (t *T) CommitInvalid() error {
	return t.rawCommit(rawconfig.T{}, "", false)
}

func (t *T) CommitTo(configPath string) error {
	return t.rawCommit(rawconfig.T{}, configPath, true)
}

func (t *T) CommitToInvalid(configPath string) error {
	return t.rawCommit(rawconfig.T{}, configPath, false)
}

func (t *T) CommitData(configData rawconfig.T) error {
	return t.rawCommit(configData, "", true)
}

func (t *T) CommitDataTo(configData rawconfig.T, configPath string) error {
	return t.rawCommit(configData, configPath, true)
}

func (t *T) CommitDataToInvalid(configData rawconfig.T, configPath string) error {
	return t.rawCommit(configData, configPath, false)
}

func (t T) postCommit() error {
	if t.Referrer == nil {
		return nil
	}
	return nil
}

func (t T) DeleteSections(sections []string) error {
	deleted := 0
	for _, section := range sections {
		if _, err := t.file.GetSection(section); err != nil {
			continue
		}
		t.file.DeleteSection(section)
		deleted++
	}
	if deleted > 0 {
		t.Commit()
	}
	return nil
}

func (t T) ModTime() time.Time {
	return file.ModTime(t.ConfigFilePath)
}
