//go:build windows

package main

import _ "embed"

//go:embed windows/llc.exe
var llcExe []byte

//go:embed windows/opt.exe
var optExe []byte
