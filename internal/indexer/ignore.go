package indexer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type ignorePattern struct {
	dirOnly  bool
	anchored bool
	pattern  string
}

// IgnoreList holds patterns loaded from ignore files. Zero value ignores nothing.
type IgnoreList struct {
	patterns []ignorePattern
}

// LoadIgnoreFile appends patterns from one file. A missing file is not an error.
func (il *IgnoreList) LoadIgnoreFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		p := ignorePattern{}
		if strings.HasSuffix(trimmed, "/") {
			p.dirOnly = true
			trimmed = trimmed[:len(trimmed)-1]
		}
		if strings.Contains(trimmed, "/") {
			p.anchored = true
		}
		p.pattern = trimmed
		il.patterns = append(il.patterns, p)
	}
	return scanner.Err()
}

// ShouldIgnore returns true if the entry should be excluded.
// relPath is relative to the indexed root; isDir distinguishes dirs from files.
func (il *IgnoreList) ShouldIgnore(relPath string, isDir bool) bool {
	for _, p := range il.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		var matched bool
		var err error
		if p.anchored {
			matched, err = filepath.Match(p.pattern, relPath)
		} else {
			matched, err = filepath.Match(p.pattern, filepath.Base(relPath))
		}
		if err == nil && matched {
			return true
		}
	}
	return false
}

// LoadIgnoreFiles loads .cercleignore then .gitignore from root (both optional).
func LoadIgnoreFiles(root string) (*IgnoreList, error) {
	il := &IgnoreList{}
	if err := il.LoadIgnoreFile(filepath.Join(root, ".cercleignore")); err != nil {
		return nil, err
	}
	if err := il.LoadIgnoreFile(filepath.Join(root, ".gitignore")); err != nil {
		return nil, err
	}
	return il, nil
}
