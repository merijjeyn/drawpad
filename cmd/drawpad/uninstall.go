package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// installerMarkerPrefix is the comment line `scripts/install.sh` writes
// immediately before the `export PATH="…"` line it appends. We use it to
// recognise (and only delete) lines we wrote ourselves — never anything the
// user added by hand.
const installerMarkerPrefix = "# Added by drawpad installer on "

const uninstallHelp = `drawpad uninstall — remove drawpad from your system.

Removes the running binary and reverses the PATH edit the install script
wrote in your shell rc — but only the exact line under the
'` + installerMarkerPrefix + `…' marker, so hand-edited
shell rc lines are never touched.

USAGE
  drawpad uninstall [flags]

FLAGS
  --keep-path-edits   Leave shell rc files alone; just remove the binary.
  -h, --help          Print this help and exit.
`

// runUninstall removes drawpad from the system. main() routes here when the
// first CLI arg is "uninstall".
func runUninstall(args []string, stdout, stderr io.Writer) error {
	keepPath := false
	for _, a := range args {
		switch a {
		case "-h", "--help", "help":
			fmt.Fprint(stdout, uninstallHelp)
			return nil
		case "--keep-path-edits":
			keepPath = true
		default:
			return fmt.Errorf("unknown argument: %q (try `drawpad uninstall --help`)", a)
		}
	}

	// Clean shell rc files FIRST, before removing the binary. If the rc
	// pass fails we still want the binary gone; the reverse order would
	// leave the user with both a missing binary AND lingering PATH cruft.
	if !keepPath {
		if err := cleanShellRCs(stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "drawpad: shell rc cleanup warning: %v\n", err)
		}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	// Follow symlinks so we delete the real file, not the symlink target.
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	// On macOS/Linux, removing a running executable works fine — the file
	// is unlinked but the inode stays alive until this process exits.
	if err := os.Remove(self); err != nil {
		return fmt.Errorf("remove binary at %s: %w (try re-running with sudo)", self, err)
	}
	fmt.Fprintf(stdout, "✓ removed %s\n", self)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Uninstalled. Open a new terminal so PATH changes take effect.")
	return nil
}

func cleanShellRCs(stdout, stderr io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	candidates := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}
	for _, rc := range candidates {
		info, err := os.Stat(rc)
		if err != nil || info.IsDir() {
			continue
		}
		removed, err := scrubInstallerLines(rc)
		if err != nil {
			fmt.Fprintf(stderr, "drawpad: skipping %s: %v\n", rc, err)
			continue
		}
		if removed > 0 {
			fmt.Fprintf(stdout, "✓ removed %d installer-added line(s) from %s\n", removed, rc)
		}
	}
	return nil
}

// scrubInstallerLines removes every `installerMarkerPrefix` comment line and
// the single line immediately after each. Mirrors the awk logic from the
// (now-deprecated) scripts/uninstall.sh.
func scrubInstallerLines(path string) (int, error) {
	in, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	var kept []string
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	skipNext := false
	removed := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipNext {
			skipNext = false
			removed++
			continue
		}
		if strings.HasPrefix(line, installerMarkerPrefix) {
			skipNext = true
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if removed == 0 {
		return 0, nil
	}

	// Atomic-ish write: write to a sibling temp file, then rename. Avoids
	// truncating the user's rc if we crash mid-write.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".drawpad-uninstall-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	// Clean up tmp file on any error path.
	defer os.Remove(tmpName)

	w := bufio.NewWriter(tmp)
	for _, l := range kept {
		if _, err := w.WriteString(l + "\n"); err != nil {
			tmp.Close()
			return 0, err
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	// Preserve the original file's mode (e.g. 0644).
	if info, err := os.Stat(path); err == nil {
		_ = os.Chmod(tmpName, info.Mode())
	}
	if err := os.Rename(tmpName, path); err != nil {
		return 0, err
	}
	return removed, nil
}
