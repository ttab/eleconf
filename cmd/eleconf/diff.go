package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/urfave/cli/v3"
)

func diffAction(_ context.Context, cmd *cli.Command) error {
	var dirA, dirB string

	switch cmd.Args().Len() {
	case 1:
		dirA = "."
		dirB = cmd.Args().Get(0)
	case 2:
		dirA = cmd.Args().Get(0)
		dirB = cmd.Args().Get(1)
	default:
		return fmt.Errorf("usage: eleconf diff [<dir-a>] <dir-b>")
	}

	if err := checkDir(dirA); err != nil {
		return err
	}

	if err := checkDir(dirB); err != nil {
		return err
	}

	filesA, err := listHCLFiles(dirA)
	if err != nil {
		return fmt.Errorf("list %q: %w", dirA, err)
	}

	filesB, err := listHCLFiles(dirB)
	if err != nil {
		return fmt.Errorf("list %q: %w", dirB, err)
	}

	setA := make(map[string]bool, len(filesA))
	for _, f := range filesA {
		setA[f] = true
	}

	setB := make(map[string]bool, len(filesB))
	for _, f := range filesB {
		setB[f] = true
	}

	addCol := color.New(color.FgGreen)
	removeCol := color.New(color.FgRed)
	updateCol := color.New(color.FgYellow)

	fmt.Println("Legend:")
	_, _ = addCol.Printf("  + exists in %q but not %q\n", dirA, dirB)
	_, _ = removeCol.Printf("  - exists in %q but not %q\n", dirB, dirA)
	_, _ = updateCol.Printf("  ~ differs between the two\n")
	fmt.Println()

	var changed []string

	for _, name := range filesA {
		if !setB[name] {
			_, _ = addCol.Printf("+ %s\n", name)

			continue
		}

		equal, fErr := filesEqual(
			filepath.Join(dirA, name),
			filepath.Join(dirB, name))
		if fErr != nil {
			return fErr
		}

		if !equal {
			_, _ = updateCol.Printf("~ %s\n", name)

			changed = append(changed, name)
		}
	}

	for _, name := range filesB {
		if !setA[name] {
			_, _ = removeCol.Printf("- %s\n", name)
		}
	}

	fmt.Println()

	headerCol := color.New(color.Bold)
	hunkCol := color.New(color.FgCyan)

	for _, name := range changed {
		_, _ = headerCol.Printf("===== %s =====\n", name)

		err := printUnifiedDiff(
			filepath.Join(dirB, name),
			filepath.Join(dirA, name),
			addCol, removeCol, hunkCol)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("not a directory: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}

	return nil
}

func listHCLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		if filepath.Ext(e.Name()) == ".hcl" {
			names = append(names, e.Name())
		}
	}

	sort.Strings(names)

	return names, nil
}

func filesEqual(pathA, pathB string) (bool, error) {
	a, err := os.ReadFile(pathA)
	if err != nil {
		return false, fmt.Errorf("read %q: %w", pathA, err)
	}

	b, err := os.ReadFile(pathB)
	if err != nil {
		return false, fmt.Errorf("read %q: %w", pathB, err)
	}

	return bytes.Equal(a, b), nil
}

func printUnifiedDiff(
	fromPath, toPath string,
	addCol, removeCol, hunkCol *color.Color,
) error {
	fromData, err := os.ReadFile(fromPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", fromPath, err)
	}

	toData, err := os.ReadFile(toPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", toPath, err)
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(fromData)),
		B:        difflib.SplitLines(string(toData)),
		FromFile: fromPath,
		ToFile:   toPath,
		Context:  3,
	}

	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Errorf("compute diff: %w", err)
	}

	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			_, _ = addCol.Println(line)
		case strings.HasPrefix(line, "-"):
			_, _ = removeCol.Println(line)
		case strings.HasPrefix(line, "@@"):
			_, _ = hunkCol.Println(line)
		default:
			fmt.Println(line)
		}
	}

	return nil
}
