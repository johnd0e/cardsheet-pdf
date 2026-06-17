package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type conflictMode int

const (
	conflictAsk conflictMode = iota
	conflictOverwrite
	conflictRename
)

func parseExtractArgs(args []string) (string, string, conflictMode, bool) {
	fs := flagSet("cardsheet extract")
	outDir := fs.String("out-dir", ".", "Output directory")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	rename := fs.Bool("rename", false, "Rename on conflicts")
	if err := fs.Parse(args); err != nil {
		return "", "", conflictAsk, false
	}
	if *overwrite && *rename {
		fmt.Fprintln(os.Stderr, "input error: --overwrite and --rename are mutually exclusive")
		return "", "", conflictAsk, false
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardsheet extract [--out-dir DIR] [--overwrite | --rename] input.pdf")
		return "", "", conflictAsk, false
	}
	mode := conflictAsk
	if *overwrite {
		mode = conflictOverwrite
	} else if *rename {
		mode = conflictRename
	}
	return fs.Arg(0), *outDir, mode, true
}

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func writeExtractedImage(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func newStdinReader() *bufio.Reader {
	return bufio.NewReader(os.Stdin)
}

func resolveConflict(path string, mode conflictMode, reader *bufio.Reader) (string, bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, true, nil
	}
	if err != nil {
		return "", false, err
	}
	switch mode {
	case conflictOverwrite:
		return path, true, nil
	case conflictRename:
		return renamedPath(path), true, nil
	}
	if !isInteractive() {
		return "", false, fmt.Errorf("%s exists; use --overwrite or --rename", path)
	}
	fmt.Printf("%s exists (%s, modified %s). overwrite/rename/skip? [o/r/s]: ",
		path, formatBytes(info.Size()), info.ModTime().Format("2006-01-02 15:04:05"))
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "", false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "o", "overwrite":
		return path, true, nil
	case "r", "rename":
		return renamedPath(path), true, nil
	default:
		return path, false, nil
	}
}

func renamedPath(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
