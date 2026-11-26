package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type deployConfig struct {
	App       string `json:"app"`
	Host      string `json:"host"`
	User      string `json:"user"`
	Domain    string `json:"domain"`
	Arch      string `json:"arch"`
	RemoteDir string `json:"remote_dir"`
	Port      int    `json:"port"`
}

// cmdBuild compiles .got templates and produces a stripped, zero-CGO production single binary.
func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	targetOS := fs.String("os", runtime.GOOS, "target GOOS (e.g. linux, windows, darwin)")
	targetArch := fs.String("arch", runtime.GOARCH, "target GOARCH (e.g. amd64, arm64)")
	outPath := fs.String("o", "", "output binary path (defaults to bin/app)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if compiled, err := compileGot("."); err != nil {
		return fmt.Errorf(".got compile failed: %w", err)
	} else if compiled > 0 {
		fmt.Printf("[rapide] compiled %d .got template(s)\n", compiled)
	}

	if err := runViteBuild("."); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	dest := *outPath
	if dest == "" {
		name := "app"
		if *targetOS == "windows" {
			name += ".exe"
		}
		dest = filepath.Join("bin", name)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	fmt.Printf("[rapide] building %s/%s -> %s ...\n", *targetOS, *targetArch, dest)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", dest, ".")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+*targetOS,
		"GOARCH="+*targetArch,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	if info, err := os.Stat(dest); err == nil {
		fmt.Printf("[rapide] built %s (%.2f MB)\n", dest, float64(info.Size())/(1024*1024))
	}
	return nil
}

// cmdDeployInit scaffolds ultra-lightweight deployment files (Dockerfile, Caddyfile, systemd unit, deploy.json).
func cmdDeployInit(args []string) error {
	cwd, _ := os.Getwd()
	appName := filepath.Base(cwd)
	if len(args) > 0 && args[0] != "" {
		appName = args[0]
	}

	data := tplData{Name: appName, Module: appName}
	files := []struct {
		dest string
		tpl  string
	}{
		{"Dockerfile", "dockerfile.tmpl"},
		{"Caddyfile", "caddyfile.tmpl"},
		{filepath.Join("deploy", appName+".service"), "systemd.tmpl"},
		{"deploy.json", "deploy_json.tmpl"},
	}
	for _, f := range files {
		if err := renderTemplate(f.dest, f.tpl, data); err != nil {
			return err
		}
		fmt.Println("Created", f.dest)
	}
	fmt.Println("\nEdit deploy.json with your VPS host/domain, then run: rapide deploy")
	return nil
}

// cmdDeploy performs a 1-command zero-downtime single-binary deploy over SSH/SCP with `/up` health check.
func cmdDeploy(_ []string) error {
	raw, err := os.ReadFile("deploy.json")
	if err != nil {
		return fmt.Errorf("missing deploy.json (run `rapide deploy:init` first): %w", err)
	}
	var cfg deployConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid deploy.json: %w", err)
	}
	if cfg.Host == "" || cfg.Host == "your-vps-ip-or-host" {
		return fmt.Errorf("please configure `host` in deploy.json before deploying")
	}
	if cfg.User == "" {
		cfg.User = "root"
	}
	if cfg.Arch == "" {
		cfg.Arch = "amd64"
	}
	if cfg.RemoteDir == "" {
		cfg.RemoteDir = "/opt/" + cfg.App
	}

	releaseID := time.Now().UTC().Format("20060102150405")
	localBin := filepath.Join("bin", cfg.App+"-linux-"+cfg.Arch)
	if err := cmdBuild([]string{"--os", "linux", "--arch", cfg.Arch, "-o", localBin}); err != nil {
		return err
	}

	remoteTarget := fmt.Sprintf("%s@%s", cfg.User, cfg.Host)
	releasePath := fmt.Sprintf("%s/releases/%s", cfg.RemoteDir, releaseID)

	fmt.Printf("[rapide] uploading binary to %s:%s ...\n", remoteTarget, releasePath)
	if err := runSSH(remoteTarget, fmt.Sprintf("mkdir -p %s/releases %s/storage", cfg.RemoteDir, cfg.RemoteDir)); err != nil {
		return err
	}
	scp := exec.Command("scp", localBin, fmt.Sprintf("%s:%s", remoteTarget, releasePath))
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return fmt.Errorf("scp upload failed: %w", err)
	}

	// Atomically swap symlink and restart systemd service (triggers graceful SIGTERM drain).
	activateCmd := fmt.Sprintf(
		"chmod +x %s && ln -sfn %s %s/current && sudo systemctl restart %s",
		releasePath, releasePath, cfg.RemoteDir, cfg.App,
	)
	fmt.Println("[rapide] activating release and restarting service...")
	if err := runSSH(remoteTarget, activateCmd); err != nil {
		return err
	}

	if cfg.Domain != "" && cfg.Domain != "example.com" {
		healthURL := "https://" + cfg.Domain + "/up"
		fmt.Printf("[rapide] verifying health at %s ...\n", healthURL)
		if err := waitHealth(healthURL, 15*time.Second); err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
	}

	fmt.Printf("[rapide] deployed %s (release %s) successfully!\n", cfg.App, releaseID)
	return nil
}

func runSSH(target, remoteCmd string) error {
	cmd := exec.Command("ssh", target, remoteCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s did not return 200 OK within %s", url, timeout)
}
