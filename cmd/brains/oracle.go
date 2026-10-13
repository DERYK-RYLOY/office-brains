// Oracle: strict third-party validation via isolated portable Python.
// `brains oracle setup` auto-downloads everything (portable Python 3.12 +
// pinned pip packages) into .oracle/. `brains oracle check` opens files with
// python-docx / openpyxl / python-pptx: anything THEY reject is our bug.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	oraclePyVersion = "3.12.9"
	oraclePyURL     = "https://www.python.org/ftp/python/3.12.9/python-3.12.9-embed-amd64.zip"
	oracleGetPipURL = "https://bootstrap.pypa.io/get-pip.py"
)

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

func oracleRoot() string {
	for _, base := range []string{".", exeDir()} {
		p := filepath.Join(base, ".oracle", "python", "python.exe")
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(filepath.Join(base, ".oracle"))
			return abs
		}
	}
	abs, _ := filepath.Abs(".oracle")
	return abs
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func oraclePython() string { return filepath.Join(oracleRoot(), "python", "python.exe") }

func oracleScript(name string) string {
	for _, base := range []string{".", exeDir()} {
		p := filepath.Join(base, "oracle", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("oracle", name)
}

func cmdOracleCheck(args []string) {
	fs := flag.NewFlagSet("oracle check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	one := fs.String("file", "", "file to check (or pass positionals)")
	human := fs.Bool("human", false, "human output instead of JSON")
	_ = fs.Parse(args)
	files := []string{}
	if *one != "" {
		files = append(files, *one)
	}
	files = append(files, fs.Args()...)
	if len(files) == 0 {
		emitErr("ERR_ORACLE_ARGS", nil)
		logHuman("usage: brains oracle check --file F [more...]")
		os.Exit(2)
	}
	py := oraclePython()
	script := oracleScript("verify.py")
	if _, err := os.Stat(py); err != nil {
		emitErr("ERR_ORACLE_MISSING", nil)
		logHuman("oracle python missing. Run: brains oracle setup")
		os.Exit(1)
	}
	cmdArgs := append([]string{script}, files...)
	cmd := exec.Command(py, cmdArgs...)
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
	fs := flag.NewFlagSet("oracle setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	human := fs.Bool("human", false, "human output instead of JSON")
	_ = fs.Parse(args)
	if runtime.GOOS != "windows" {
		emitErr("ERR_ORACLE_OS", nil)
		logHuman("oracle setup is Windows-only today (needs the Windows embeddable Python build)")
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
		steps = append(steps, "downloaded portable python")
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
		if out, err := exec.Command(pyExe, getPip, "--no-warn-script-location").CombinedOutput(); err != nil {
			emitErr("ERR_ORACLE_SETUP", fmt.Errorf("%v: %s", err, tailLines(string(out), 3)))
			os.Exit(1)
		}
		_ = os.Remove(getPip)
		steps = append(steps, "installed pip")
	} else {
		steps = append(steps, "portable python present")
	}
	req := oracleScript("requirements.txt")
	if out, err := exec.Command(pyExe, "-m", "pip", "install", "--no-cache-dir", "-r", req).CombinedOutput(); err != nil {
		emitErr("ERR_ORACLE_SETUP", fmt.Errorf("%v: %s", err, tailLines(string(out), 5)))
		os.Exit(1)
	}
	steps = append(steps, "installed requirements.txt")
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
