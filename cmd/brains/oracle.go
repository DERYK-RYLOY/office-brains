// Oracle: strict third-party validation via isolated portable Python.
// `brains oracle setup` auto-downloads everything (portable Python 3.12 +
// pinned, hash-checked pip packages) into .oracle/. `brains oracle check`
// opens files with python-docx / openpyxl / python-pptx: anything THEY
// reject is our bug.
//
// Binary-only installs work: the scripts and requirement files are
// embedded in the binary and materialized into .oracle/ when no repo
// checkout is present. Downloads are SHA-256 pinned (override only via
// BRAINS_ORACLE_ALLOW_UNPINNED=1) and pip installs run in hash-checking
// mode (--require-hashes).
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DERYK-RYLOY/office-brains/oracle"
)

const (
	oraclePyVersion = "3.12.9"
	oraclePyURL     = "https://www.python.org/ftp/python/3.12.9/python-3.12.9-embed-amd64.zip"
	oraclePySHA256  = "615861fb801e8b04c847598db4e1e46e4b046295017caa37cb5486dde72b5865"
	oracleGetPipURL = "https://bootstrap.pypa.io/get-pip.py"
	oracleGetPipSHA = "356e0d0de21cdc348c3394084d90939b85e4246c4d14c76aee6ca4f46646ea98"
)

// stringList is a repeatable string flag (e.g. --file a --file b).
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("empty value")
	}
	*s = append(*s, v)
	return nil
}

func allowUnpinned() bool { return os.Getenv("BRAINS_ORACLE_ALLOW_UNPINNED") == "1" }

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifySHA256(path, want string) error {
	got, err := sha256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", filepath.Base(path), got, want)
	}
	return nil
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// findUpward walks from start toward the filesystem root looking for
// rel (a directory or file). Returns the absolute base dir that contains it.
func findUpward(start, rel string) string {
	cur, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for i := 0; i < 64; i++ {
		if _, err := os.Stat(filepath.Join(cur, rel)); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
	return ""
}

func existingOracleRoot() string {
	if env := os.Getenv("BRAINS_ORACLE_ROOT"); env != "" {
		p := filepath.Join(env, "python", "python.exe")
		if runtime.GOOS != "windows" {
			p = filepath.Join(env, "python", "bin", "python3")
		}
		if _, err := os.Stat(p); err == nil {
			return env
		}
		return env // env wins even if not yet installed; setup will fill it
	}
	for _, start := range []string{".", exeDir()} {
		if base := findUpward(start, filepath.Join(".oracle", "python", "python.exe")); base != "" {
			return filepath.Join(base, ".oracle")
		}
	}
	return ""
}

func oracleRoot() string {
	if found := existingOracleRoot(); found != "" {
		return found
	}
	// Fresh setup: next to the binary when we can write there (e.g.
	// ~/.office-brains/bin), otherwise the current directory.
	if exeDir() != "" && probeWritable(filepath.Join(exeDir(), ".oracle-probe")) {
		_ = os.RemoveAll(filepath.Join(exeDir(), ".oracle-probe"))
		return filepath.Join(exeDir(), ".oracle")
	}
	abs, _ := filepath.Abs(".oracle")
	return abs
}

func probeWritable(probe string) bool {
	if err := os.MkdirAll(probe, 0o755); err != nil {
		return false
	}
	f := filepath.Join(probe, "w")
	if err := os.WriteFile(f, []byte("ok"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(f)
	return true
}

func oraclePython() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(oracleRoot(), "python", "python.exe")
	}
	return filepath.Join(oracleRoot(), "python", "bin", "python3")
}

// materialize returns a path to the named oracle asset, preferring a repo
// checkout on disk (dev mode) and falling back to the embedded copy
// (binary installs), extracting it under the oracle root if needed.
func materialize(root, name string) (string, error) {
	for _, start := range []string{".", exeDir()} {
		if base := findUpward(start, filepath.Join("oracle", name)); base != "" {
			return filepath.Join(base, "oracle", name), nil
		}
	}
	dir := filepath.Join(root, "oracle")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, name)
	data, err := oracle.FS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("embedded asset %s: %w", name, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func cmdOracle(args []string) {
	if len(args) < 1 {
		emitErr("ERR_ORACLE_ARGS", nil)
		logHuman("usage: brains oracle setup|check [--file F] [files...]")
		os.Exit(2)
	}
	switch args[0] {
	case "setup":
		cmdOracleSetup(args[1:])
	case "check":
		cmdOracleCheck(args[1:])
	default:
		emitErr("ERR_ORACLE_CMD", nil)
		logHuman("usage: brains oracle setup|check")
		os.Exit(2)
	}
}

func cmdOracleCheck(args []string) {
	fs := newFlagSet("oracle check")
	var one stringList
	fs.Var(&one, "file", "file to check (repeatable; positional args also accepted)")
	human := fs.Bool("human", false, "human output instead of JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	files := append([]string{}, one...)
	files = append(files, fs.Args()...)
	if len(files) == 0 {
		emitErr("ERR_ORACLE_ARGS", nil)
		logHuman("usage: brains oracle check [--file F]... [more files]")
		os.Exit(2)
	}
	py := oraclePython()
	root := oracleRoot()
	var pyArgs []string
	script, err := materialize(root, "verify.py")
	if err != nil {
		emitErr("ERR_ORACLE_MATERIALIZE", err)
		os.Exit(1)
	}
	pyArgs = append([]string{script}, files...)
	if _, err := os.Stat(py); err != nil {
		// Portable Python missing: fall back to a system interpreter on
		// non-Windows, or tell the user to run setup.
		if runtime.GOOS != "windows" {
			if sys, serr := exec.LookPath("python3"); serr == nil {
				py = sys
			} else if sys, serr := exec.LookPath("python"); serr == nil {
				py = sys
			}
		}
		if py == oraclePython() && runtime.GOOS == "windows" {
			emitErr("ERR_ORACLE_MISSING", nil)
			logHuman("oracle python missing. Run: brains oracle setup")
			os.Exit(1)
		}
	}
	cmd := exec.Command(py, pyArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		emitErr("ERR_ORACLE_RUN", err)
		logHuman("%s", strings.TrimSpace(stderr.String()))
		os.Exit(1)
	}
	var rep map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		emitErr("ERR_ORACLE_PARSE", err)
		os.Exit(1)
	}
	if *human {
		ok, _ := rep["ok"].(bool)
		status := "FAIL"
		if ok {
			status = "PASS"
		}
		printHumanOK("oracle check", map[string]string{"result": status, "files": fmt.Sprint(len(files))})
		if !ok {
			os.Exit(1)
		}
		return
	}
	emitJSON(rep)
	if ok, _ := rep["ok"].(bool); !ok {
		os.Exit(1)
	}
}

func cmdOracleSetup(args []string) {
	fs := newFlagSet("oracle setup")
	human := fs.Bool("human", false, "human output instead of JSON")
	_ = fs.Parse(args)
	if runtime.GOOS != "windows" {
		emitErr("ERR_ORACLE_OS", nil)
		logHuman("oracle setup is Windows-only today (needs the Windows embeddable Python build).")
		logHuman("On Linux/macOS: pip install -r oracle/requirements-hashes.txt, then oracle check will find python3 on PATH.")
		os.Exit(2)
	}
	root := oracleRoot()
	pyDir := filepath.Join(root, "python")
	pyExe := filepath.Join(pyDir, "python.exe")
	steps := []string{}
	download := func(url, dest string) error {
		client := &http.Client{Timeout: 300 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("GET %s: %s", url, resp.Status)
		}
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, resp.Body)
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		return err
	}
	if _, err := os.Stat(pyExe); err != nil {
		logHuman("oracle setup: downloading portable Python %s...", oraclePyVersion)
		if err := os.MkdirAll(pyDir, 0o755); err != nil {
			emitErr("ERR_ORACLE_SETUP", err)
			os.Exit(1)
		}
		zipPath := filepath.Join(root, "pyembed.zip")
		if err := download(oraclePyURL, zipPath); err != nil {
			emitErr("ERR_ORACLE_DOWNLOAD", err)
			os.Exit(1)
		}
		if err := verifySHA256(zipPath, oraclePySHA256); err != nil {
			if !allowUnpinned() {
				_ = os.Remove(zipPath)
				emitErr("ERR_ORACLE_HASH", err)
				logHuman("pyembed.zip failed integrity check. Re-run or set BRAINS_ORACLE_ALLOW_UNPINNED=1 to bypass (not recommended).")
				os.Exit(1)
			}
			logHuman("WARN: pyembed.zip hash mismatch ignored (BRAINS_ORACLE_ALLOW_UNPINNED=1)")
		}
		steps = append(steps, "downloaded portable python (sha256 verified)")
		if err := unzipTo(zipPath, pyDir); err != nil {
			emitErr("ERR_ORACLE_SETUP", err)
			os.Exit(1)
		}
		_ = os.Remove(zipPath)
		// enable site packages in embeddable python
		pth := filepath.Join(pyDir, "python312._pth")
		if b, err := os.ReadFile(pth); err == nil {
			s := strings.Replace(string(b), "#import site", "import site", 1)
			_ = os.WriteFile(pth, []byte(s), 0o644)
		}
		steps = append(steps, "extracted portable python")
		getPip := filepath.Join(root, "get-pip.py")
		if err := download(oracleGetPipURL, getPip); err != nil {
			emitErr("ERR_ORACLE_DOWNLOAD", err)
			os.Exit(1)
		}
		if err := verifySHA256(getPip, oracleGetPipSHA); err != nil {
			if !allowUnpinned() {
				_ = os.Remove(getPip)
				emitErr("ERR_ORACLE_HASH", err)
				logHuman("get-pip.py failed integrity check. Re-run or set BRAINS_ORACLE_ALLOW_UNPINNED=1 to bypass (not recommended).")
				os.Exit(1)
			}
			logHuman("WARN: get-pip.py hash mismatch ignored (BRAINS_ORACLE_ALLOW_UNPINNED=1)")
		}
		steps = append(steps, "downloaded get-pip.py (sha256 verified)")
		if out, err := exec.Command(pyExe, getPip, "--no-warn-script-location").CombinedOutput(); err != nil {
			emitErr("ERR_ORACLE_SETUP", fmt.Errorf("%v: %s", err, tailLines(string(out), 3)))
			os.Exit(1)
		}
		_ = os.Remove(getPip)
		steps = append(steps, "installed pip")
	} else {
		steps = append(steps, "portable python present")
	}
	req, err := materialize(root, "requirements-hashes.txt")
	if err != nil {
		emitErr("ERR_ORACLE_MATERIALIZE", err)
		os.Exit(1)
	}
	if out, err := exec.Command(pyExe, "-m", "pip", "install", "--no-cache-dir", "--require-hashes", "-r", req).CombinedOutput(); err != nil {
		emitErr("ERR_ORACLE_SETUP", fmt.Errorf("%v: %s", err, tailLines(string(out), 5)))
		os.Exit(1)
	}
	steps = append(steps, "installed requirements (hash-checked)")
	data := map[string]any{"root": root, "steps": steps, "python": oraclePyVersion}
	if *human {
		printHumanOK("oracle setup", map[string]string{"root": root})
		return
	}
	emitJSON(map[string]any{"ok": true, "command": "oracle setup", "data": data})
}

func unzipTo(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		p := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(p, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
