package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	crypt "github.com/rapide-go/rapide/encryption"
)

type tplData struct {
	Name       string
	Snake      string
	Plural     string
	Module     string
	Timestamp  string
	StructName string
}

func cmdNew(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide new <app-name> [--svelte|--react|--api] [--db sqlite|postgres|mysql]")
	}
	name := args[0]
	preset := "svelte"
	dbDriver := "sqlite"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--svelte":
			preset = "svelte"
		case "--react":
			preset = "react"
		case "--api":
			preset = "api"
		case "--db":
			if i+1 < len(args) {
				i++
				dbDriver = args[i]
			}
		}
	}

	dirs := []string{
		filepath.Join(name, "app", "models"),
		filepath.Join(name, "app", "http", "controllers"),
		filepath.Join(name, "app", "http", "middleware"),
		filepath.Join(name, "app", "http", "requests"),
		filepath.Join(name, "app", "providers"),
		filepath.Join(name, "bootstrap"),
		filepath.Join(name, "config"),
		filepath.Join(name, "database", "migrations"),
		filepath.Join(name, "routes"),
		filepath.Join(name, "storage", "logs"),
	}
	if preset != "api" {
		dirs = append(dirs,
			filepath.Join(name, "resources", "js", "Pages"),
			filepath.Join(name, "resources", "css"),
			filepath.Join(name, "public"),
		)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	data := tplData{Name: name, Module: name}
	if err := renderTemplate(filepath.Join(name, "go.mod"), "go_mod.tmpl", data); err != nil {
		return err
	}
	if err := renderTemplate(filepath.Join(name, ".env"), "env.tmpl", data); err != nil {
		return err
	}
	if err := renderTemplate(filepath.Join(name, "main.go"), "app_main.tmpl", data); err != nil {
		return err
	}
	fmt.Printf("Created Rapide app %s/ (preset: %s, db: %s)\nNext: cd %s && rapide key:generate && rapide serve\n", name, preset, dbDriver, name)
	return nil
}

// cmdKeyGenerate writes a cryptographically random 32-byte APP_KEY into .env.
func cmdKeyGenerate() error {
	key, err := crypt.GenerateKey()
	if err != nil {
		return err
	}
	const envPath = ".env"
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			if werr := os.WriteFile(envPath, []byte("APP_KEY="+key+"\n"), 0o644); werr != nil {
				return werr
			}
			fmt.Println("Application key set in .env")
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "APP_KEY=") {
			lines[i] = "APP_KEY=" + key
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, "APP_KEY="+key)
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	fmt.Println("Application key set successfully.")
	return nil
}

// cmdForwardToApp forwards project commands (migrate, route:list, queue:work, etc.)
// directly to the current app via `go run . <cmd> ...`.
func cmdForwardToApp(cmdName string, args []string) error {
	fullArgs := append([]string{"run", ".", cmdName}, args...)
	c := exec.Command("go", fullArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func cmdMakeModel(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide make:model <Name> [-m] [-c]")
	}
	name := toPascal(args[0])
	snake := toSnake(name)
	plural := pluralize(snake)

	withMig, withCtrl := false, false
	for _, flag := range args[1:] {
		if flag == "-m" {
			withMig = true
		}
		if flag == "-c" {
			withCtrl = true
		}
	}

	data := tplData{Name: name, Snake: snake, Plural: plural}
	path := filepath.Join("app", "models", snake+".go")
	if err := renderTemplate(path, "model.tmpl", data); err != nil {
		return err
	}
	fmt.Println("Created", path)

	if withMig {
		if err := cmdMakeMigration([]string{"create_" + plural + "_table"}); err != nil {
			return err
		}
	}
	if withCtrl {
		if err := cmdMakeController([]string{name}); err != nil {
			return err
		}
	}
	return nil
}

func cmdMakeController(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide make:controller <Name>")
	}
	base := strings.TrimSuffix(toPascal(args[0]), "Controller")
	data := tplData{
		Name:       base,
		StructName: base + "Controller",
		Snake:      toSnake(base),
		Plural:     pluralize(toSnake(base)),
	}
	path := filepath.Join("app", "http", "controllers", data.Snake+"_controller.go")
	if err := renderTemplate(path, "controller.tmpl", data); err != nil {
		return err
	}
	fmt.Println("Created", path)
	return nil
}

func cmdMakeMigration(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide make:migration <name>")
	}
	name := toSnake(args[0])
	ts := time.Now().Format("20060102150405")
	data := tplData{
		Name:       name,
		Timestamp:  ts,
		StructName: toPascal(name),
	}
	path := filepath.Join("database", "migrations", fmt.Sprintf("%s_%s.go", ts, name))
	if err := renderTemplate(path, "migration.tmpl", data); err != nil {
		return err
	}
	fmt.Println("Created", path)
	return nil
}

func cmdMakeProvider(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide make:provider <Name>")
	}
	base := strings.TrimSuffix(toPascal(args[0]), "Provider")
	data := tplData{
		Name:       base,
		StructName: base + "Provider",
		Snake:      toSnake(base),
	}
	path := filepath.Join("app", "providers", data.Snake+"_provider.go")
	if err := renderTemplate(path, "provider.tmpl", data); err != nil {
		return err
	}
	fmt.Println("Created", path)
	return nil
}

func cmdMakeJob(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rapide make:job <Name>")
	}
	name := toPascal(args[0])
	data := tplData{
		Name:       name,
		StructName: name,
		Snake:      toSnake(name),
	}
	path := filepath.Join("app", "jobs", data.Snake+".go")
	if err := renderTemplate(path, "job.tmpl", data); err != nil {
		return err
	}
	fmt.Println("Created", path)
	return nil
}
