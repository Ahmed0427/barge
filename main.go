package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-git/go-git/v6"
)

func CloneRepo(repoURL string) (string, error) {
	clonePath, err := os.MkdirTemp("", "barge_clone_*")
	if err != nil {
		return clonePath, fmt.Errorf("failed to make temp dir for cloning: %w", err)
	}

	_, err = git.PlainClone(clonePath, &git.CloneOptions{
		URL:          repoURL,
		Progress:     os.Stderr,
		SingleBranch: true,
		Depth:        1,
	})
	if err != nil {
		return clonePath, fmt.Errorf("failed to clone the repo: %w", err)
	}

	return clonePath, nil
}

func main() {
	var repoURL string
	flag.StringVar(&repoURL, "r", "", "git repository URL")
	flag.Parse()
	if repoURL == "" {
		fmt.Fprintln(os.Stderr, "you have to provide a repository URL")
		flag.Usage()
		os.Exit(1)
	}

	clonePath, err := CloneRepo(repoURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Clone Path: %v\n", clonePath)
}
