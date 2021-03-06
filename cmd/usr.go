package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subUsr = &cobra.Command{
		Use:   "usr",
		Short: "Manage users",
		Long: ` A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
	subUsrPrint = &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
)

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdEdit             commands.CmdObjectEdit
		cmdEditConfig       commands.CmdObjectEditConfig
		cmdEval             commands.CmdObjectEval
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdSet              commands.CmdObjectSet
		cmdStatus           commands.CmdObjectStatus
		cmdUnset            commands.CmdObjectUnset
	)

	kind := "usr"
	head := subUsr
	subPrint := subUsrPrint
	root := rootCmd

	root.AddCommand(head)
	head.AddCommand(subPrint)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEdit.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, cmdEdit.Command, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPrintConfig.Init(kind, subPrint, &selectorFlag)
	cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
