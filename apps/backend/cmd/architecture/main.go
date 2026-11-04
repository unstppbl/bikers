// Command architecture checks the internal package boundaries approved in AGENTS.md.
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("architecture", flag.ContinueOnError)
	flags.SetOutput(errOut)
	root := flags.String("root", ".", "backend module directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "usage: architecture [-root backend-directory]")
		return 2
	}
	violations, err := check(*root)
	if err != nil {
		fmt.Fprintf(errOut, "architecture: %v\n", err)
		return 2
	}
	if len(violations) != 0 {
		for _, violation := range violations {
			fmt.Fprintln(errOut, violation)
		}
		return 1
	}
	fmt.Fprintln(out, "Architecture check passed (all internal Go files, including tests and build tags).")
	return 0
}

func check(root string) ([]string, error) {
	module, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	internalBase := module + "/internal"
	prefix := internalBase + "/"
	var violations []string
	err = filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ownerPath := strings.TrimPrefix(filepath.ToSlash(filepath.Dir(rel)), "internal/")
		owner := layer(ownerPath)
		if owner == "" {
			violations = append(violations, fmt.Sprintf("%s: package path internal/%s is outside the approved layout", rel, ownerPath))
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", rel, err)
			}
			if importPath != internalBase && !strings.HasPrefix(importPath, prefix) {
				continue
			}
			targetPath := strings.TrimPrefix(importPath, prefix)
			target := layer(targetPath)
			position := positions.Position(spec.Pos())
			if target == "" {
				violations = append(violations, fmt.Sprintf("%s:%d: import %q is outside the approved layout", rel, position.Line, importPath))
				continue
			}
			// An external owner_test package references the owner it tests. All
			// its other internal dependencies still follow the owner's matrix.
			selfTest := strings.HasSuffix(path, "_test.go") && strings.HasSuffix(file.Name.Name, "_test") && targetPath == ownerPath
			if owner != "" && !selfTest && !mayImport(owner, target) {
				violations = append(violations, fmt.Sprintf("%s:%d: internal/%s must not import internal/%s (see AGENTS.md dependency direction)", rel, position.Line, ownerPath, targetPath))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(violations)
	return violations, nil
}

func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		value := fields[1]
		if strings.HasPrefix(value, "\"") {
			value, err = strconv.Unquote(value)
		}
		if err != nil || value == "" {
			return "", fmt.Errorf("invalid module directive in go.mod")
		}
		return value, nil
	}
	return "", fmt.Errorf("module directive missing from go.mod")
}

func layer(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		switch path {
		case "models", "config", "database", "workers", "app":
			return path
		}
	}
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	switch parts[0] {
	case "managers", "clients":
		if !lowercaseName(parts[1]) {
			return ""
		}
		return parts[0]
	case "resources":
		return parts[0]
	case "database":
		return "repository"
	}
	return ""
}

func lowercaseName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if char < 'a' || char > 'z' {
			return false
		}
	}
	return true
}

func mayImport(owner, target string) bool {
	switch owner {
	case "app":
		return true
	case "database":
		return target == "models"
	case "repository":
		return target == "database" || target == "models"
	case "managers":
		return target == "models" || target == "database" || target == "managers"
	case "clients":
		return target == "models"
	case "resources":
		return target == "managers" || target == "models"
	case "workers":
		// No queue client exists today. Telegram is not a queue-polling
		// dependency. Register a real queue client here with its package map
		// when a worker is implemented; this is not an all-client exemption.
		return target == "managers" || target == "models"
	default:
		return false
	}
}
