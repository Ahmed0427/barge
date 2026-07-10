package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

func CloneRepo(repoURL string) (string, error) {
	clonePath, err := os.MkdirTemp("", "barge_clone_*")
	if err != nil {
		return "", fmt.Errorf("failed to make temp dir: %w", err)
	}

	err = runCommand("", "git", "clone", "--depth", "1", repoURL, clonePath)
	if err != nil {
		return clonePath, fmt.Errorf("failed to clone repo: %w", err)
	}

	return clonePath, nil
}

func main() {
	var repoURL string
	flag.StringVar(&repoURL, "r", "", "Git repository URL")
	flag.Parse()

	if repoURL == "" {
		fmt.Fprintln(os.Stderr, "Error: You must provide a Git repository URL.")
		os.Exit(1)
	}

	clonePath, err := CloneRepo(repoURL)
	if clonePath != "" {
		defer os.RemoveAll(clonePath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Clone error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(filepath.Join(clonePath, "Dockerfile")); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "\nError: No Dockerfile found in the repository.")
		fmt.Fprintln(os.Stderr, "Please provide a repo with a Dockerfile to make stuff simpler.")
		os.Exit(1)
	}

	buildTag := "latest"
	appID := strings.ToLower(rand.Text())
	imageName := fmt.Sprintf("barge-%s:%s", appID, buildTag)

	if err := runCommand(clonePath, "docker", "build", "-t", imageName, "."); err != nil {
		fmt.Fprintf(os.Stderr, "Docker build error: %v\n", err)
		os.Exit(1)
	}
}
