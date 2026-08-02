package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

type App struct {
	Name      string `json:"name"`
	RepoURL   string `json:"repo_url"`
	Image     string `json:"image"`
	Container string `json:"container"`
	Port      int    `json:"port"`
	ClonePath string `json:"clone_path"`
}

type State struct {
	Apps []App `json:"apps"`
}

var stateFile = filepath.Join(os.Getenv("HOME"), ".barge", "state.json")

func loadState() (*State, error) {
	s := &State{}
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("corrupt state file: %w", err)
	}
	return s, nil
}

func saveState(s *State) error {
	os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

func findApp(s *State, name string) (App, int) {
	for i, a := range s.Apps {
		if a.Name == name {
			return a, i
		}
	}
	return App{}, -1
}

func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdOutput(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", ee, ee.Stderr)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func deployCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: barge deploy <app-name> [git-url]")
	}
	appName := args[0]
	var repoURL string
	if len(args) >= 2 {
		repoURL = args[1]
	}

	state, err := loadState()
	if err != nil {
		return err
	}

	existing, idx := findApp(state, appName)
	if repoURL == "" {
		if idx == -1 {
			return fmt.Errorf("no previous repo URL for %q – please provide a git URL", appName)
		}
		repoURL = existing.RepoURL
	} else if idx != -1 && existing.RepoURL != repoURL {
		color.Yellow("Repo URL changed, removing old clone and container.")
		if existing.Container != "" {
			runCmd("", "docker", "rm", "-f", existing.Container)
		}
		if existing.ClonePath != "" {
			os.RemoveAll(existing.ClonePath)
		}
		existing = App{}
		idx = -1
	}

	var clonePath string
	if idx != -1 && existing.ClonePath != "" {
		clonePath = existing.ClonePath
		color.Green("Updating existing repo...")
		if err := runCmd(clonePath, "git", "pull", "--ff-only"); err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
	} else {
		color.Green("Cloning repository...")
		clonePath = filepath.Join(os.TempDir(), "barge_"+appName)
		os.RemoveAll(clonePath)
		if err := runCmd("", "git", "clone", "--depth", "1", repoURL, clonePath); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	if _, err := os.Stat(filepath.Join(clonePath, "Dockerfile")); os.IsNotExist(err) {
		return errors.New("no Dockerfile found")
	}

	// 3. Build image
	imageTag := fmt.Sprintf("barge/%s:latest", appName)
	color.Green("Building Docker image %s...", imageTag)
	if err := runCmd(clonePath, "docker", "build", "-t", imageTag, "."); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// 4. Stop/remove old container (by name)
	containerName := "barge-" + appName
	if existing.Container != "" {
		color.Yellow("Removing old container %s...", existing.Container)
		runCmd("", "docker", "rm", "-f", existing.Container)
	}

	port := 0
	if idx != -1 && existing.Port != 0 {
		port = existing.Port
	} else {
		port, err = findFreePort()
		if err != nil {
			return fmt.Errorf("cannot find free port: %w", err)
		}
	}

	color.Green("Starting container %s on port %d...", containerName, port)
	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", port, 80),
		"--restart", "unless-stopped",
		imageTag,
	}
	_, err = runCmdOutput("", "docker", runArgs...)
	if err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	updated := App{
		Name:      appName,
		RepoURL:   repoURL,
		Image:     imageTag,
		Container: containerName,
		Port:      port,
		ClonePath: clonePath,
	}
	if idx != -1 {
		state.Apps[idx] = updated
	} else {
		state.Apps = append(state.Apps, updated)
	}
	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	color.Cyan("\n🚢 App %q deployed successfully!", appName)
	color.Cyan("   Access it at: http://localhost:%d", port)
	return nil
}

func logsCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: barge logs <app-name>")
	}
	appName := args[0]
	state, err := loadState()
	if err != nil {
		return err
	}
	app, idx := findApp(state, appName)
	if idx == -1 {
		return fmt.Errorf("app %q not found", appName)
	}
	if app.Container == "" {
		return fmt.Errorf("app %q is not running", appName)
	}

	cmd := exec.Command("docker", "logs", "-f", app.Container)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: barge stop <app-name>")
	}
	appName := args[0]
	state, err := loadState()
	if err != nil {
		return err
	}
	app, idx := findApp(state, appName)
	if idx == -1 {
		return fmt.Errorf("app %q not found", appName)
	}
	if app.Container == "" {
		return fmt.Errorf("app %q is not running", appName)
	}
	color.Yellow("Stopping container %s...", app.Container)
	if err := runCmd("", "docker", "stop", app.Container); err != nil {
		return err
	}
	color.Green("App %q stopped.", appName)
	return nil
}

func startCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: barge start <app-name>")
	}
	appName := args[0]
	state, err := loadState()
	if err != nil {
		return err
	}
	app, idx := findApp(state, appName)
	if idx == -1 {
		return fmt.Errorf("app %q not found", appName)
	}
	if app.Container == "" {
		return fmt.Errorf("app %q has no container record", appName)
	}
	color.Green("Starting container %s...", app.Container)
	if err := runCmd("", "docker", "start", app.Container); err != nil {
		return err
	}
	color.Cyan("App %q started. Access it at http://localhost:%d", appName, app.Port)
	return nil
}

func deleteCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: barge delete <app-name>")
	}
	appName := args[0]
	state, err := loadState()
	if err != nil {
		return err
	}
	app, idx := findApp(state, appName)
	if idx == -1 {
		return fmt.Errorf("app %q not found", appName)
	}

	if app.Container != "" {
		color.Yellow("Removing container %s...", app.Container)
		runCmd("", "docker", "rm", "-f", app.Container)
	}
	if app.Image != "" {
		color.Yellow("Removing image %s...", app.Image)
		runCmd("", "docker", "rmi", "-f", app.Image)
	}
	if app.ClonePath != "" {
		color.Yellow("Removing clone at %s...", app.ClonePath)
		os.RemoveAll(app.ClonePath)
	}

	state.Apps = append(state.Apps[:idx], state.Apps[idx+1:]...)
	if err := saveState(state); err != nil {
		return err
	}
	color.Green("App %q deleted.", appName)
	return nil
}

func listCommand(args []string) error {
	state, err := loadState()
	if err != nil {
		return err
	}
	if len(state.Apps) == 0 {
		fmt.Println("No apps deployed yet.")
		return nil
	}
	color.Cyan("Deployed apps:\n")
	for _, a := range state.Apps {
		fmt.Printf("  %-15s port: %-5d repo: %s\n", a.Name, a.Port, a.RepoURL)
	}
	return nil
}

const msg = `
Barge - a tiny PaaS

Commands:
  deploy <app-name> [git-url]   Deploy or update an app
  logs <app-name>               Stream app logs
  stop <app-name>               Stop a running app
  start <app-name>              Start a stopped app
  delete <app-name>             Remove app completely
  list                          Show all apps
`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, msg[1:])
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var err error
	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "deploy":
		err = deployCommand(cmdArgs)
	case "logs":
		err = logsCommand(cmdArgs)
	case "stop":
		err = stopCommand(cmdArgs)
	case "start":
		err = startCommand(cmdArgs)
	case "delete":
		err = deleteCommand(cmdArgs)
	case "list":
		err = listCommand(cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}
}
