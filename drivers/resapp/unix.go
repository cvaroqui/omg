// +build !windows

package resapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/xexec"
)

// T is the driver structure for app unix & linux.
type T struct {
	BaseT
	Path         path.T         `json:"path"`
	Nodes        []string       `json:"nodes"`
	ScriptPath   string         `json:"script"`
	StartCmd     string         `json:"start"`
	StopCmd      string         `json:"stop"`
	CheckCmd     string         `json:"check"`
	InfoCmd      string         `json:"info"`
	StatusLogKw  bool           `json:"status_log"`
	CheckTimeout *time.Duration `json:"check_timeout"`
	InfoTimeout  *time.Duration `json:"info_timeout"`
	Cwd          string         `json:"cwd"`
	User         string         `json:"user"`
	Group        string         `json:"group"`
	LimitAs      *int64         `json:"limit_as"`
	LimitCpu     *time.Duration `json:"limit_cpu"`
	LimitCore    *int64         `json:"limit_core"`
	LimitData    *int64         `json:"limit_data"`
	LimitFSize   *int64         `json:"limit_fsize"`
	LimitMemLock *int64         `json:"limit_memlock"`
	LimitNoFile  *int64         `json:"limit_nofile"`
	LimitNProc   *int64         `json:"limit_nproc"`
	LimitRss     *int64         `json:"limit_rss"`
	LimitStack   *int64         `json:"limit_stack"`
	LimitVMem    *int64         `json:"limit_vmem"`
}

type LoggerCheck struct {
	*xexec.LoggerExec
	R *T
}

func (w LoggerCheck) DoOut(s xexec.Bytetexter, pid int) {
	w.LoggerExec.DoOut(s, pid)
	w.R.StatusLog().Info(s.Text())
}

func (w LoggerCheck) DoErr(s xexec.Bytetexter, pid int) {
	w.LoggerExec.DoErr(s, pid)
	w.R.StatusLog().Warn(s.Text())
}

func (t T) SortKey() string {
	if len(t.StartCmd) > 1 && isSequenceNumber(t.StartCmd) {
		return t.StartCmd + " " + t.RID()
	} else {
		return t.RID() + " " + t.RID()
	}
}

func (t T) Abort() bool {
	return false
}

// Stop the Resource
func (t T) Stop() (err error) {
	t.Log().Debug().Msg("Stop()")
	var xcmd xexec.T
	if xcmd, err = t.PrepareXcmd(t.StopCmd, "stop"); err != nil {
		return
	} else if len(xcmd.CmdArgs) == 0 {
		return
	}
	cmd := exec.Command(xcmd.CmdArgs[0], xcmd.CmdArgs[1:]...)
	if err = xcmd.Update(cmd); err != nil {
		return
	}
	appStatus := t.Status()
	if appStatus == status.Down {
		t.Log().Info().Msg("already down")
		return nil
	}
	c := xexec.NewCmd(t.Log(), cmd, xexec.NewLoggerExec(t.Log(), zerolog.InfoLevel, zerolog.WarnLevel))
	if timeout := t.GetTimeout("stop"); timeout > 0 {
		c.SetDuration(timeout)
	}
	t.Log().Info().Msgf("running %s", cmd.String())
	if err = c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	t.Log().Debug().Msg("status()")
	var xcmd xexec.T
	var err error
	if xcmd, err = t.PrepareXcmd(t.CheckCmd, "status"); err != nil {
		t.Log().Error().Err(err).Msg("PrepareXcmd")
		if t.StatusLogKw {
			t.StatusLog().Error("prepareXcmd %v", err.Error())
		}
		return status.Undef
	} else if len(xcmd.CmdArgs) == 0 {
		return status.NotApplicable
	}
	cmd := exec.Command(xcmd.CmdArgs[0], xcmd.CmdArgs[1:]...)
	if err = xcmd.Update(cmd); err != nil {
		return status.Undef
	}
	var watcher interface{}
	defaultWatcher := xexec.NewLoggerExec(t.Log(), zerolog.DebugLevel, zerolog.DebugLevel)
	if t.StatusLogKw {
		watcher = &LoggerCheck{LoggerExec: defaultWatcher, R: t}
	} else {
		watcher = defaultWatcher
	}
	c := xexec.NewCmd(t.Log(), cmd, watcher)
	if timeout := t.GetTimeout("check"); timeout > 0 {
		c.SetDuration(timeout)
	}
	t.Log().Debug().Msgf("Status() running %s", cmd.String())
	if err = c.Start(); err != nil {
		return status.Undef
	}
	if err = c.Wait(); err != nil {
		t.Log().Debug().Msg("status is down")
		return status.Down
	}
	t.Log().Debug().Msgf("status is up")
	return status.Up
}

func (t T) Provision() error {
	return nil
}

func (t T) Unprovision() error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// PrepareXcmd returns xexec.T for action string 's'
// It prepare xexec.T CmdArgs for xexec.T.Update(cmd)
func (t T) PrepareXcmd(s string, action string) (c xexec.T, err error) {
	if len(s) == 0 {
		t.Log().Debug().Msgf("no command for action '%v'", action)
		return
	}
	var baseCommand string
	if baseCommand, err = t.getCmdStringFromBoolRule(s, action); err != nil {
		return
	}
	if len(baseCommand) == 0 {
		t.Log().Debug().Msgf("no command for action '%v'", action)
		return
	}
	limitCommands := xexec.ShLimitCommands(t.toLimits())
	if len(limitCommands) > 0 {
		baseCommand = limitCommands + " && " + baseCommand
	}
	if c.CmdArgs, err = xexec.CommandArgsFromString(baseCommand); err != nil {
		t.Log().Error().Err(err).Msgf("unable to CommandArgsFromString for action '%v'", action)
		return
	}
	if c.CmdEnv, err = t.getEnv(); err != nil {
		t.Log().Error().Err(err).Msgf("unable to create command environment for action '%v'", action)
		return
	}
	t.Log().Debug().Msgf("env for action '%v': '%v'", action, c.CmdEnv)
	if c.Credential, err = xexec.Credential(t.User, t.Group); err != nil {
		t.Log().Error().Err(err).Msgf("unable to set credential from user '%v', group '%v' for action '%v'", t.User, t.Group, action)
		return
	}
	if t.Cwd != "" {
		t.Log().Debug().Msgf("set command Dir to '%v'", t.Cwd)
		c.Cwd = t.Cwd
	}
	return
}

// getCmdStringFromBoolRule get command string for 'action' using bool rule on 's'
// if 's' is a
//   true like => getScript() + " " + action
//   false like => ""
//   other => original value
func (t T) getCmdStringFromBoolRule(s string, action string) (string, error) {
	if scriptCommandBool, ok := boolRule(s); ok {
		switch scriptCommandBool {
		case true:
			scriptValue := t.getScript()
			if scriptValue == "" {
				t.Log().Warn().Msgf("action '%v' as true value but 'script' keyword is empty", action)
				return "", fmt.Errorf("unable to get script value")
			}
			return scriptValue + " " + action, nil
		case false:
			return "", nil
		}
	}
	return s, nil
}

// getScript return script kw value
// when script is a basename:
//   <pathetc>/namespaces/<namespace>/<kind>/<svcname>.d/<script> (when namespace is not root)
//   <pathetc>/<svcname>.d/<script> (when namespace is root)
//
func (t T) getScript() string {
	s := t.ScriptPath
	if len(s) == 0 {
		return ""
	}
	if s[0] == os.PathSeparator {
		return s
	}
	var p string
	if t.Path.Namespace != "root" {
		p = fmt.Sprintf("%s/namespaces/%s/%s/%s.d/%s", rawconfig.Node.Paths.Etc, t.Path.Namespace, t.Path.Kind, t.Path.Name, s)
	} else {
		p = fmt.Sprintf("%s/%s.d/%s", rawconfig.Node.Paths.Etc, t.Path.Name, s)
	}
	return filepath.FromSlash(p)
}

// boolRule return bool, ok
// detect if s is a bool like, or sequence number
func boolRule(s string) (bool, bool) {
	if v, err := converters.Bool.Convert(s); err == nil {
		return v.(bool), true
	}
	if isSequenceNumber(s) {
		return true, true
	}
	return false, false
}

func isSequenceNumber(s string) bool {
	if len(s) < 2 {
		return false
	}
	if _, err := strconv.ParseInt(s, 10, 16); err == nil {
		return true
	}
	return false
}

func (t T) GetTimeout(action string) time.Duration {
	var timeout *time.Duration
	switch action {
	case "start":
		timeout = t.StartTimeout
	case "stop":
		timeout = t.StopTimeout
	case "check":
		timeout = t.CheckTimeout
	case "info":
		timeout = t.InfoTimeout
	}
	if timeout == nil {
		timeout = t.Timeout
	}
	if timeout == nil {
		return 0
	}
	return *timeout
}
