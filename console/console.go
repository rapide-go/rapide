package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
	"text/tabwriter"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
)

// Command is implemented by any built-in, application, or third-party Artisan CLI command.
type Command interface {
	Name() string
	Description() string
	Run(ctx context.Context, args []string) error
}

// FuncCommand is a convenient struct adapter to define a Command inline without a new type.
type FuncCommand struct {
	CmdName string
	CmdDesc string
	Action  func(ctx context.Context, args []string) error
}

func (f FuncCommand) Name() string        { return f.CmdName }
func (f FuncCommand) Description() string { return f.CmdDesc }
func (f FuncCommand) Run(ctx context.Context, args []string) error {
	return f.Action(ctx, args)
}

// Kernel manages and dispatches registered Artisan CLI commands.
type Kernel struct {
	mu       sync.RWMutex
	commands map[string]Command
	out      io.Writer
}

// NewKernel creates an empty console Kernel writing help output to stdout.
func NewKernel() *Kernel {
	return &Kernel{
		commands: make(map[string]Command),
		out:      os.Stdout,
	}
}

// Register adds one or more commands to the Kernel.
func (k *Kernel) Register(cmds ...Command) *Kernel {
	k.mu.Lock()
	for _, c := range cmds {
		if c != nil && c.Name() != "" {
			k.commands[c.Name()] = c
		}
	}
	k.mu.Unlock()
	return k
}

// Execute runs the command named in args[0] (defaulting to defaultCmd if args is empty).
func (k *Kernel) Execute(ctx context.Context, args []string, defaultCmd string) error {
	name := defaultCmd
	var cmdArgs []string
	if len(args) > 0 {
		name = args[0]
		cmdArgs = args[1:]
	}
	if name == "" || name == "help" || name == "list" || name == "-h" || name == "--help" {
		k.PrintHelp()
		return nil
	}

	k.mu.RLock()
	cmd, ok := k.commands[name]
	k.mu.RUnlock()
	if !ok {
		k.PrintHelp()
		return fmt.Errorf("console: unknown command %q", name)
	}
	return cmd.Run(ctx, cmdArgs)
}

// PrintHelp outputs a sorted table of all registered commands and their descriptions.
func (k *Kernel) PrintHelp() {
	k.mu.RLock()
	names := make([]string, 0, len(k.commands))
	for n := range k.commands {
		names = append(names, n)
	}
	slices.Sort(names)

	w := tabwriter.NewWriter(k.out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Available commands:")
	for _, n := range names {
		fmt.Fprintf(w, "  %s\t%s\n", n, k.commands[n].Description())
	}
	k.mu.RUnlock()
	_ = w.Flush()
}

// Add registers commands into an App's console Kernel (usable from any Service Provider).
func Add(app *foundation.App, cmds ...Command) {
	foundation.AddExtension[Command](app, cmds...)
	if k, err := container.TryResolve[*Kernel](app.Container); err == nil && k != nil {
		k.Register(cmds...)
	}
}

// From returns the App's console *Kernel, creating and hydrating it with all provider commands if needed.
func From(app *foundation.App) *Kernel {
	k, err := container.TryResolve[*Kernel](app.Container)
	if err != nil || k == nil {
		k = NewKernel()
		container.Bind[*Kernel](app.Container, k)
	}
	for _, cmd := range foundation.Extensions[Command](app) {
		k.Register(cmd)
	}
	return k
}
