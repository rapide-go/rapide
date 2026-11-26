package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "new":
		err = cmdNew(args)
	case "serve", "dev":
		err = cmdServe(args)
	case "build":
		err = cmdBuild(args)
	case "deploy:init":
		err = cmdDeployInit(args)
	case "deploy":
		err = cmdDeploy(args)
	case "compile":
		n, cerr := compileGot(".")
		if cerr == nil {
			fmt.Printf("Compiled %d.got template(s)\n", n)
		}
		err = cerr
	case "key:generate":
		err = cmdKeyGenerate()
	case "add":
		err = cmdAdd(args)
	case "remove":
		err = cmdRemove(args)
	case "package:discover":
		err = cmdPackageDiscover(args)
	case "make:model":
		err = cmdMakeModel(args)
	case "make:controller":
		err = cmdMakeController(args)
	case "make:migration":
		err = cmdMakeMigration(args)
	case "make:provider":
		err = cmdMakeProvider(args)
	case "make:job":
		err = cmdMakeJob(args)
	// Unified project CLI forwarding (runs `go run. <cmd>...`)
	case "migrate", "migrate:rollback", "migrate:fresh", "migrate:status",
		"route:list", "routes", "queue:work", "schedule:list", "schedule:work", "cache:clear":
		if cmd == "routes" {
			cmd = "route:list"
		}
		err = cmdForwardToApp(cmd, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Rapide CLI — modern Go Generics Framework

Usage:
  rapide new <app> [--svelte|--react|--api] [--db sqlite|postgres|mysql]
  rapide serve / rapide dev             Run Go hot-reload + Vite dev server together
  rapide build [--os linux --arch amd64] Build frontend (Vite) + stripped zero-CGO single binary
  rapide deploy:init                    Generate Dockerfile, Caddyfile, systemd unit, and deploy.json
  rapide deploy                         Zero-downtime single-binary SSH deploy with /up health check
  rapide key:generate                   Generate a 32-byte APP_KEY in.env
  rapide migrate[:rollback|:fresh|:status] Run database migrations
  rapide route:list / rapide routes     List all registered HTTP routes
  rapide queue:work                     Run standalone background queue workers
  rapide schedule:list                  List registered scheduled tasks
  rapide add / remove <module>          Manage Go packages & auto-discover providers
  rapide make:model <Name> [-m] [-c]    Generate a model (with optional migration & controller)
  rapide make:controller <Name>         Generate a RESTful resource controller
  rapide make:migration <name>          Generate a timestamped migration
  rapide make:provider <Name>           Generate a service provider
  rapide make:job <Name>                Generate a background job`)
}
