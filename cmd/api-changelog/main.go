package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
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
	logPrefix     = "api-changelog: "
)

func main() {
	log.SetFlags(0)
	log.SetPrefix(logPrefix)

	base := flag.String("base", defaultBase, "git ref the head is compared with")
	head := flag.String("head", workingTree, "git ref compared with the base; HEAD reads the spec from the working tree")
	specPath := flag.String("spec", defaultSpec, "swagger.json to compare")
	out := flag.String("out", "", "changelog file path (default: <spec dir>/<name>.md)")
	name := flag.String("name", "", "changelog name (default: head branch)")
	flag.Parse()

	ctx := context.Background()
	if *name == "" {
		branch, err := git(ctx, "rev-parse", "--abbrev-ref", *head)
		if err != nil {
			log.Fatal(err)
		}
		if branch == "" || branch == detachedHead {
			log.Fatalf("%s is not a branch: check out a branch or pass -name", *head)
		}
		*name = branch
	}
	path := *out
	if path == "" {
		path = filepath.Join(filepath.Dir(*specPath), strings.ReplaceAll(*name, "/", "-")+changelogExt)
	}

	baseJSON, headJSON, short, err := readSpecs(ctx, *base, *head, *specPath)
	if err != nil {
		log.Fatal(err)
	}

	sections, err := changelog.Render(baseJSON, headJSON)
	if err != nil {
		log.Fatal(err)
	}

	if sections == "" {
		fmt.Println(noChanges + *base)
		err = os.Remove(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
		}
		return
	}
	header := fmt.Sprintf("# API changes: %s\n\nCompared with `%s` at merge base `%s`.", *name, *base, short)
	doc := header + partSeparator + sections + "\n"
	err = os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", path)
	err = os.WriteFile(path, []byte(doc), fileMode) //nolint:gosec // output path is chosen by the developer
	if err != nil {
		log.Fatal(err)
	}
}

func readSpecs(ctx context.Context, base, head, specPath string) ([]byte, []byte, string, error) {
	mergeBase, err := git(ctx, "merge-base", head, base)
	if err != nil {
		return nil, nil, "", err
	}
	short, err := git(ctx, "rev-parse", "--short", mergeBase)
	if err != nil {
		return nil, nil, "", err
	}

	specInRepo := filepath.ToSlash(filepath.Clean(specPath))
	baseJSON, err := gitRaw(ctx, "show", mergeBase+":"+specInRepo)
	if err != nil {
		return nil, nil, "", err
	}
	var headJSON []byte
	if head == workingTree {
		headJSON, err = os.ReadFile(specPath) //nolint:gosec // the spec path comes from the developer running the tool
	} else {
		headJSON, err = gitRaw(ctx, "show", head+":"+specInRepo)
	}
	if err != nil {
		return nil, nil, "", err
	}
	return baseJSON, headJSON, short, nil
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
