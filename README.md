rebuild-gobin rebuilds binaries under GOBIN if they were built with a
different Go version.

Tool first scans $GOBIN directory (defaults to $GOPATH/bin or $HOME/go/bin)
with the "go version -m" command to figure out module version and Go version
for each binary, then runs "go install path@version" for each command's
path.

For example, if there's a httpstat binary inside a GOBIN directory, then "go
version -m $(which httpstat)" outputs something like this:

    ~ ¶ go version -m go/bin/httpstat
    go/bin/httpstat: devel +4de4480dc3 Fri Dec 4 22:08:54 2020 +0000
        path    github.com/davecheney/httpstat
        mod github.com/davecheney/httpstat  v1.0.0  h1:3o8oiYGB4AKsammYvME8tWywgLPTGUl6H75LTsKoO7w=
        dep github.com/fatih/color  v1.10.0 h1:s36xzo75JdqLaaWoiEHk767eHiwo0598uUxyfiPkDsg=
        dep github.com/mattn/go-colorable   v0.1.8  h1:c1ghPdyEDarC70ftn0y+A/Ee++9zz8ljHG1b13eJ0s8=
        dep github.com/mattn/go-isatty  v0.0.12 h1:wuysRhFDzyxgEmMf5xjvJ2M9dZoWAXNNr5LSBS7uHXY=
        dep golang.org/x/net    v0.0.0-20201202161906-c7110b5ffcbb  h1:eBmm0M9fYhWpKZLjQUUKka/LtIxf46G4fxeEz5KJr9U=
        dep golang.org/x/sys    v0.0.0-20200930185726-fdedc70b468f  h1:+Nyd8tzPX9R7BWHguqsrbFdRx3WQ/1ib8I44HXV5yTA=
        dep golang.org/x/text   v0.3.3  h1:cokOdA+Jmi5PJGXLlLllQSgYigAEfHXJAERHVMaCc2k=

rebuild-gobin will then run "go install
github.com/davecheney/httpstat@v1.0.0" if it detects that version reported
by "go version" differs from one that binary was built with.

If you run this tool with "-u" flag, then it will call "go install
path@latest" for each binary, forcing their update to the latest available
version.

This tool relies on "go install" semantics introduced in Go 1.16 (see
https://github.com/golang/go/issues/40276 for more details).
