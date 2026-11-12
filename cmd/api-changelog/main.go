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
	defaultSpec   = "docs/swagger.json"
	changelogExt  = ".md"
	detachedHead  = "HEAD"
	dirMode       = 0o755
	fileMode      = 0o644
	noChanges     = "no API changes against "
	partSeparator = "\n\n"
)

func main() {
	base := flag.String("base", defaultBase, "git ref the branch is compared with")
	specPath := flag.String("spec", defaultSpec, "swagger.json to compare")
	out := flag.String("out", "", "changelog file path (default: <spec dir>/<name>.md)")
	name := flag.String("name", "", "changelog name (default: current branch)")
	flag.Parse()
	if err := run(*base, *specPath, *out, *name); err != nil {
		fmt.Fprintln(os.Stderr, "api-changelog:", err)
		os.Exit(1)
	}
}

func run(base, specPath, path, name string) error {
	ctx := context.Background()
	if name == "" {
		branch, err := git(ctx, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		if branch == detachedHead {
			return errors.New("detached HEAD: check out a branch or pass -name")
		}
		name = branch
	}
	if path == "" {
		path = filepath.Join(filepath.Dir(specPath), strings.ReplaceAll(name, "/", "-")+changelogExt)
	}
	mergeBase, err := git(ctx, "merge-base", "HEAD", base)
	if err != nil {
		return err
	}
	short, err := git(ctx, "rev-parse", "--short", mergeBase)
	if err != nil {
		return err
	}
	baseJSON, err := gitRaw(ctx, "show", mergeBase+":"+filepath.ToSlash(filepath.Clean(specPath)))
	if err != nil {
		return err
	}
	headJSON, err := os.ReadFile(specPath) //nolint:gosec // the spec path comes from the developer running the tool
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
