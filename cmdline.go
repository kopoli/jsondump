package main

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/kopoli/appkit"
)

var cmdParseRegex = regexp.MustCompile(`[A-Za-z0-9][-A-Za-z0-9]*`)

type Command struct {
	Cmd         []string
	Help        string
	Flags       *flag.FlagSet
	subCommands []*Command
}

func HasFlags(fs *flag.FlagSet) bool {
	ret := false
	fs.VisitAll(func(f *flag.Flag) {
		ret = true
	})
	return ret
}

func SplitCommand(cmdstr string) []string {
	return strings.Fields(cmdstr)
}

func SplitArguments(argstr string) []string {
	return strings.Split(argstr, "\000")
}

func JoinArguments(args []string) string {
	return strings.Join(args, "\000")
}

func NewCommand(parent *Command, cmd string, help string) *Command {
	cmds := []string{""}
	if len(cmd) > 0 {
		cmds = strings.Fields(cmd)

		for i := range cmds {
			if !cmdParseRegex.MatchString(cmds[i]) {
				s := fmt.Sprintf("Error: Could not parse command: %s", cmds[i])
				panic(s)
			}
		}
	}
	flags := flag.NewFlagSet(cmds[0], flag.ContinueOnError)

	ret := &Command{
		Cmd:   cmds,
		Help:  help,
		Flags: flags,
	}

	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintf(out, "Command: %s\n\n%s\n",
			strings.Join(ret.Cmd, ", "), ret.Help)
		if HasFlags(flags) {
			fmt.Fprintf(out, "\nOptions:\n")
			flags.PrintDefaults()
		}
	}

	if parent != nil {
		parent.subCommands = append(parent.subCommands, ret)
	}
	return ret
}

func (c *Command) Parse(args []string, opts appkit.Options) error {
	var err error
	if c.Flags != nil {
		err = c.Flags.Parse(args)
		if err != nil {
			return err
		}
	}

	args = c.Flags.Args()

	cmd := ""
	if c.Cmd[0] != "" {
		cmd = opts.Get("cmdline-command", "") + " " + c.Cmd[0]
		cmd = strings.TrimLeft(cmd, " ")
	}

	opts.Set("cmdline-command", cmd)
	opts.Set("cmdline-args", JoinArguments(args))

	if len(args) == 0 {
		return nil
	}

	for _, sc := range c.subCommands {
		for i := range sc.Cmd {
			if sc.Cmd[i] == args[0] {
				err = sc.Parse(args[1:], opts)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Command) CommandList(out io.Writer) {
	wr := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	var printall func(pfx string, c *Command)
	printall = func(pfx string, c *Command) {
		if c.Cmd[0] != "" {
			fmt.Fprintf(wr, "%s%s\t-\t%s\n", pfx, strings.Join(c.Cmd, ", "), c.Help)
		}
		for _, sc := range c.subCommands {
			printall(pfx+"  ", sc)
		}
	}

	printall("", c)
	wr.Flush()
}
