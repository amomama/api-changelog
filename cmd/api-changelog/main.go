package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amomama/api-changelog"
)

const (
	defaultBase   = "origin/HEAD"
	workingTree   = "HEAD"
	defaultSpec   = "docs/swagger.json"
	changelogExt  = ".md"
	detachedHead  = "HEAD"
	dirMode       = 0o755
	fileMode      = 0o644
	noChanges     = "no API changes against "
	partSeparator = "\n\n"
)

func main() {
	base := flag.String("base", defaultBase, "git ref the head is compared with")
	head := flag.String("head", workingTree, "git ref compared with the base; HEAD reads the spec from the working tree")
	specPath := flag.String("spec", defaultSpec, "swagger.json to compare")
	out := flag.String("out", "", "changelog file path (default: <spec dir>/<name>.md)")
	name := flag.String("name", "", "changelog name (default: head branch)")
	flag.Parse()
	if err := run(*base, *head, *specPath, *out, *name); err != nil {
		fmt.Fprintln(os.Stderr, "api-changelog:", err)
		os.Exit(1)
	}
}

func run(base, head, specPath, path, name string) error {
	ctx := context.Background()
	if name == "" {
		branch, err := git(ctx, "rev-parse", "--abbrev-ref", head)
		if err != nil {
			return err
		}
		if branch == "" || branch == detachedHead {
			return fmt.Errorf("%s is not a branch: check out a branch or pass -name", head)
		}
		name = branch
	}
	if path == "" {
		path = filepath.Join(filepath.Dir(specPath), strings.ReplaceAll(name, "/", "-")+changelogExt)
	}
	mergeBase, err := git(ctx, "merge-base", head, base)
	if err != nil {
		return err
	}
	short, err := git(ctx, "rev-parse", "--short", mergeBase)
	if err != nil {
		return err
	}
	specInRepo := filepath.ToSlash(filepath.Clean(specPath))
	baseJSON, err := gitRaw(ctx, "show", mergeBase+":"+specInRepo)
	if err != nil {
		return err
	}
	var headJSON []byte
	if head == workingTree {
		headJSON, err = os.ReadFile(specPath) //nolint:gosec // the spec path comes from the developer running the tool
	} else {
		headJSON, err = gitRaw(ctx, "show", head+":"+specInRepo)
	}
	if err != nil {
		return err
	}
	sections, err := changelog.Render(baseJSON, headJSON)
	if err != nil {
		return err
	}
	if sections == "" {
		return remove(path, base)
	}
	header := fmt.Sprintf("# API changes: %s\n\nCompared with `%s` at merge base `%s`.", name, base, short)
	return write(path, header+partSeparator+sections+"\n")
}

func remove(path, base string) error {
	fmt.Println(noChanges + base)
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func write(path, doc string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return os.WriteFile(path, []byte(doc), fileMode) //nolint:gosec // output path is chosen by the developer
}

func gitRaw(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // arguments are git subcommands and refs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func git(ctx context.Context, args ...string) (string, error) {
	out, err := gitRaw(ctx, args...)
	return strings.TrimSpace(string(out)), err
}
