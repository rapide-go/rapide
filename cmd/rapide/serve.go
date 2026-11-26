package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// cmdServe builds the current Rapide app, starts it with Go hot-reload,
// and automatically supervises Vite (`npm/pnpm/bun run dev`) when package.json is present.
func cmdServe(args []string) error {
	if out, err := exec.Command("go", "list", "-f", "{{.Name}}", ".").Output(); err == nil {
		if pkg := strings.TrimSpace(string(out)); pkg != "main" {
			return fmt.Errorf("current directory is package %q, not package main — run `rapide serve` inside your application directory (e.g. `cd starter && ..\\rapide serve`)", pkg)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tmpDir, err := os.MkdirTemp("", "rapide-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	binName := "app"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	// Start Vite dev server alongside Go when package.json exists (kept alive across Go restarts).
	viteCmd := startViteDev(".")
	defer stopProcess(viteCmd)

	var running *exec.Cmd
	restart := func(reason string) {
		if reason != "" {
			fmt.Printf("\n[rapide] change detected (%s) — rebuilding...\n", reason)
		} else {
			fmt.Println("[rapide] building application...")
		}

		stopProcess(running)
		running = nil

		if compiled, err := compileGot("."); err != nil {
			fmt.Printf("[rapide] .got compile failed: %v (waiting for next change)\n", err)
			return
		} else if compiled > 0 {
			fmt.Printf("[rapide] compiled %d .got template(s)\n", compiled)
		}

		build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			fmt.Printf("[rapide] build failed: %v (waiting for next change)\n", err)
			return
		}

		cmd := exec.Command(binPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			fmt.Printf("[rapide] failed to start app: %v\n", err)
			return
		}
		running = cmd
	}

	restart("")

	changes := watchDir(ctx, ".", 250*time.Millisecond, 200*time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n[rapide] shutting down dev server...")
			stopProcess(running)
			stopProcess(viteCmd)
			return nil
		case changed, ok := <-changes:
			if !ok {
				stopProcess(running)
				stopProcess(viteCmd)
				return nil
			}
			restart(changed)
		}
	}
}

func detectNodePackageManager(dir string) (string, []string) {
	if _, err := os.Stat(filepath.Join(dir, "bun.lockb")); err == nil {
		return "bun", []string{"run"}
	}
	if _, err := os.Stat(filepath.Join(dir, "bun.lock")); err == nil {
		return "bun", []string{"run"}
	}
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		return "pnpm", []string{"run"}
	}
	return "npm", []string{"run"}
}

func startViteDev(dir string) *exec.Cmd {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return nil
	}
	pm, baseArgs := detectNodePackageManager(dir)
	if _, err := exec.LookPath(pm); err != nil {
		return nil
	}
	fmt.Printf("[rapide] starting Vite dev server (%s run dev)...\n", pm)
	cmd := exec.Command(pm, append(baseArgs, "dev")...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("[rapide] warning: could not start Vite: %v\n", err)
		return nil
	}
	return cmd
}

func runViteBuild(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return nil
	}
	pm, baseArgs := detectNodePackageManager(dir)
	if _, err := exec.LookPath(pm); err != nil {
		return fmt.Errorf("%s not found in PATH for frontend build", pm)
	}
	fmt.Printf("[rapide] building frontend assets (%s run build)...\n", pm)
	cmd := exec.Command(pm, append(baseArgs, "build")...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}
