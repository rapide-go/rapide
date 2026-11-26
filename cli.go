package rapide

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/rapide-go/rapide/cache"
	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/console"
	schedule "github.com/rapide-go/rapide/console/scheduling"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/database/schema"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/typegen"
	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/queue"
)

// RunCLI dispatches built-in Artisan commands (`serve`, `route:list`, `migrate*`, `queue:work`,
// `schedule:*`, `cache:clear`) plus any custom commands registered via console.Add(app.Kernel, cmd).
func (a *App) RunCLI(args []string) error {
	allMigrations := append(a.allMigrations(), foundation.Extensions[schema.Migration](a.Kernel)...)

	cli := console.From(a.Kernel).Register(
		console.FuncCommand{
			CmdName: "serve",
			CmdDesc: "Start the HTTP server and background workers",
			Action: func(ctx context.Context, _ []string) error {
				addr := a.Addr
				if port := config.Get("app.port", 0); port > 0 && addr == ":8080" {
					addr = fmt.Sprintf(":%d", port)
				}
				applog.Info("rapide server listening",
					"app", config.Get("app.name", "Rapide"),
					"env", config.Get("app.env", "local"),
					"addr", addr,
				)
				return a.Kernel.ServeWithOptions(addr, a.Router, a.Server)
			},
		},
		console.FuncCommand{
			CmdName: "route:list",
			CmdDesc: "List all registered HTTP routes",
			Action: func(_ context.Context, _ []string) error {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "METHOD\tPATH\tNAME")
				for _, rt := range a.Router.Routes() {
					fmt.Fprintf(w, "%s\t%s\t%s\n", rt.Method, rt.Path, rt.Name)
				}
				return w.Flush()
			},
		},
		console.FuncCommand{
			CmdName: "migrate",
			CmdDesc: "Run pending database migrations",
			Action: func(ctx context.Context, _ []string) error {
				ran, err := schema.NewMigrator(query.DB(), allMigrations...).Up(ctx)
				for _, n := range ran {
					fmt.Println("Migrated:", n)
				}
				return err
			},
		},
		console.FuncCommand{
			CmdName: "migrate:rollback",
			CmdDesc: "Roll back the last migration batch",
			Action: func(ctx context.Context, _ []string) error {
				rolled, err := schema.NewMigrator(query.DB(), allMigrations...).Rollback(ctx)
				for _, n := range rolled {
					fmt.Println("Rolled back:", n)
				}
				return err
			},
		},
		console.FuncCommand{
			CmdName: "migrate:fresh",
			CmdDesc: "Reset and re-run all database migrations",
			Action: func(ctx context.Context, _ []string) error {
				ran, err := schema.NewMigrator(query.DB(), allMigrations...).Fresh(ctx)
				for _, n := range ran {
					fmt.Println("Migrated:", n)
				}
				return err
			},
		},
		console.FuncCommand{
			CmdName: "migrate:status",
			CmdDesc: "Show applied and pending migrations",
			Action: func(ctx context.Context, _ []string) error {
				list, err := schema.NewMigrator(query.DB(), allMigrations...).Status(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "STATUS\tBATCH\tMIGRATION")
				for _, st := range list {
					state := "Pending"
					if st.Applied {
						state = "Ran"
					}
					fmt.Fprintf(w, "%s\t%d\t%s\n", state, st.Batch, st.Name)
				}
				return w.Flush()
			},
		},
		console.FuncCommand{
			CmdName: "queue:work",
			CmdDesc: "Run queue workers in standalone mode until SIGINT/SIGTERM",
			Action: func(ctx context.Context, _ []string) error {
				applog.Info("queue:work running", "queues", queue.Manager().Names())
				sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
				defer stop()
				<-sigCtx.Done()
				return a.Shutdown(context.Background())
			},
		},
		console.FuncCommand{
			CmdName: "schedule:list",
			CmdDesc: "List all registered scheduled tasks",
			Action: func(_ context.Context, _ []string) error {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "TASK\tINTERVAL\tNEXT RUN (UTC)\tNO OVERLAP")
				for _, t := range schedule.Schedule().Tasks() {
					fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", t.Name, t.Interval, t.NextRun.UTC().Format("2006-01-02 15:04:05"), t.NoOverlap)
				}
				return w.Flush()
			},
		},
		console.FuncCommand{
			CmdName: "schedule:work",
			CmdDesc: "Run the task scheduler in foreground until SIGINT/SIGTERM",
			Action: func(ctx context.Context, _ []string) error {
				applog.Info("schedule:work running", "tasks", len(schedule.Schedule().Tasks()))
				sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
				defer stop()
				<-sigCtx.Done()
				return a.Shutdown(context.Background())
			},
		},
		console.FuncCommand{
			CmdName: "cache:clear",
			CmdDesc: "Flush all cached entries",
			Action: func(_ context.Context, _ []string) error {
				cache.Flush()
				fmt.Println("Application cache cleared.")
				return nil
			},
		},
		console.FuncCommand{
			CmdName: "gen:ts",
			CmdDesc: "Generate TypeScript types, typed route helpers, and live-props client (types.ts, routes.ts, live.ts)",
			Action: func(_ context.Context, cmdArgs []string) error {
				dir := a.typeGenDir
				if len(cmdArgs) > 0 && cmdArgs[0] != "" {
					dir = cmdArgs[0]
				}
				if dir == "" {
					dir = "resources/js/rapide"
				}
				if err := typegen.Generate(a.Router, dir); err != nil {
					return err
				}
				fmt.Println("Generated TypeScript bindings in", dir)
				return nil
			},
		},
	)

	return cli.Execute(context.Background(), args, "serve")
}

func (a *App) allMigrations() []schema.Migration {
	_, defMigs, _, _, _ := snapshotRegistry()
	return append(defMigs, a.migrations...)
}
