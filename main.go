// rebuild-gobin rebuilds binaries under GOBIN if they were built with a Go
// version different from the currently installed.
//
// Tool first scans $GOBIN directory (defaults to $GOPATH/bin or $HOME/go/bin)
// with the "go version -m" command to figure out module version and Go version
// for each binary, then runs "go install path@version" for each command's
// path.
//
// For example, if there's a httpstat binary inside a GOBIN directory, then "go
// version -m $(which httpstat)" outputs something like this:
//
//	~ ¶ go version -m go/bin/httpstat
//	go/bin/httpstat: devel +4de4480dc3 Fri Dec 4 22:08:54 2020 +0000
//	    path    github.com/davecheney/httpstat
//	    mod github.com/davecheney/httpstat  v1.0.0  h1:3o8oiYGB4AKsammYvME8tWywgLPTGUl6H75LTsKoO7w=
//	    dep github.com/fatih/color  v1.10.0 h1:s36xzo75JdqLaaWoiEHk767eHiwo0598uUxyfiPkDsg=
//	    dep github.com/mattn/go-colorable   v0.1.8  h1:c1ghPdyEDarC70ftn0y+A/Ee++9zz8ljHG1b13eJ0s8=
//	    dep github.com/mattn/go-isatty  v0.0.12 h1:wuysRhFDzyxgEmMf5xjvJ2M9dZoWAXNNr5LSBS7uHXY=
//	    dep golang.org/x/net    v0.0.0-20201202161906-c7110b5ffcbb  h1:eBmm0M9fYhWpKZLjQUUKka/LtIxf46G4fxeEz5KJr9U=
//	    dep golang.org/x/sys    v0.0.0-20200930185726-fdedc70b468f  h1:+Nyd8tzPX9R7BWHguqsrbFdRx3WQ/1ib8I44HXV5yTA=
//	    dep golang.org/x/text   v0.3.3  h1:cokOdA+Jmi5PJGXLlLllQSgYigAEfHXJAERHVMaCc2k=
//
// rebuild-gobin will then run "go install
// github.com/davecheney/httpstat@v1.0.0" if it detects that version reported
// by "go version" differs from one that binary was built with.
//
// If you run this tool with "-u" flag, then it will call "go install
// path@latest" for each binary, forcing their update to the latest available
// version.
//
// This tool relies on the "[go install]" semantics introduced in Go 1.16.
//
// [go install]: https://go.dev/ref/mod#go-install
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)
	var upgrade bool
	flag.BoolVar(&upgrade, "u", upgrade, "reinstall programs using their '@latest' version")
	flag.Parse()
	if err := run(upgrade); err != nil {
		log.Fatal(err)
	}
}

func run(upgrade bool) error {
	gobin, err := getGobin()
	if err != nil {
		return err
	}
	programs, err := inspectGobin(gobin)
	if err != nil {
		return err
	}
	gover, err := goVersion()
	if err != nil {
		return err
	}
	var skipped []string
	var failed []string
	var tempDir string
	for _, p := range programs {
		if !upgrade && p.goVersion == gover {
			continue
		}
		if p.modVersion == "(devel)" {
			skipped = append(skipped, p.path)
			continue
		}
		if tempDir == "" {
			tempDir, err = os.MkdirTemp("", "rebuild-gobin-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tempDir)
		}
		targetVersion := p.modVersion
		if upgrade {
			targetVersion = "latest"
		}
		if err := rebuild(tempDir, fmt.Sprintf("%s@%s", p.path, targetVersion)); err != nil {
			failed = append(failed, p.path)
		}
	}
	if len(skipped) == 0 && len(failed) == 0 {
		return nil
	}
	if len(skipped) != 0 {
		log.Println("Skipped the following programs because of the (devel) module version:")
		for _, s := range skipped {
			log.Println(" ", s)
		}
	}
	if len(failed) != 0 {
		log.Println("There were errors installing the following modules, see the full log above:")
		for _, s := range failed {
			log.Println(" ", s)
		}
	}
	return nil
}

func rebuild(tempDir, spec string) error {
	if spec == "" || !strings.ContainsRune(spec, '@') {
		return fmt.Errorf("invalid path@version spec: %q", spec)
	}
	cmd := exec.Command("go", "install", spec)
	cmd.Dir = tempDir
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Println("running:", cmd)
	return cmd.Run()
}

type program struct {
	path       string
	modVersion string
	goVersion  string
}

func (p *program) empty() bool { return *p == program{} }
func (p *program) valid() bool { return p.path != "" && p.modVersion != "" && p.goVersion != "" }

func inspectGobin(dir string) ([]program, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	buf, err := exec.CommandContext(ctx, "go", "version", "-m", dir).Output()
	if err != nil {
		return nil, err
	}
	var out []program
	var current program
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	/*
		go/bin/tlstun: devel +4de4480dc3 Fri Dec 4 22:08:54 2020 +0000
		path	github.com/artyom/tlstun/v2
		mod	github.com/artyom/tlstun/v2	v2.2.1	h1:uo/Oj/63PdKuwYJ+LiAl61wefhC2CvNpDMegN+xxpmM=
		dep	github.com/armon/go-socks5	v0.0.0-20160902184237-e75332964ef5	h1:0
	*/
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, dir) {
			if current.valid() {
				out = append(out, current)
			}
			current = program{
				goVersion: text[strings.Index(text, ": ")+2:],
			}
			continue
		}
		fields := strings.Fields(text)
		if len(fields) == 2 && fields[0] == "path" {
			current.path = fields[1]
			continue
		}
		if len(fields) >= 3 && fields[0] == "mod" {
			current.modVersion = fields[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if current.valid() {
		out = append(out, current)
	}
	return out, nil
}

func getGobin() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "-json", "GOBIN", "GOPATH")
	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}
	tmp := struct{ GOBIN, GOPATH string }{}
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return "", fmt.Errorf("cannot parse go env output: %w", err)
	}
	if tmp.GOBIN != "" {
		return tmp.GOBIN, nil
	}
	return filepath.Join(tmp.GOPATH, "bin"), nil
}

func goVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	buf, err := exec.CommandContext(ctx, "go", "env", "-json", "GOOS", "GOARCH").Output()
	if err != nil {
		return "", err
	}
	tmp := struct{ GOOS, GOARCH string }{}
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return "", fmt.Errorf("cannot parse go env output: %w", err)
	}
	buf, err = exec.CommandContext(ctx, "go", "version").Output()
	if err != nil {
		return "", err
	}
	buf = bytes.TrimSpace(buf)
	buf = bytes.TrimPrefix(buf, []byte("go version"))
	buf = bytes.TrimSuffix(buf, []byte(tmp.GOOS+"/"+tmp.GOARCH))
	buf = bytes.TrimSpace(buf)
	return string(buf), nil
}
