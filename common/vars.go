package common

import (
	"os"
	"path/filepath"
)

var (
	// directory structure, for example, deployed in /opt
	// /opt
	// ├─ demo(Home)
	// │  ├─ bin(ExecDir)
	// │  │  ├─ executable(ExecPath)
	// │  ├─ conf
	// │  │  ├─ conf.yaml(CfgPath)
	// │  ├─ log
	// │  ├─ var
	// .
	ExecPath, _ = os.Executable()
	ExecDir     = filepath.Dir(ExecPath)
	Home        = filepath.Join(ExecDir, "..")
	CfgPath     = filepath.Join(Home, "conf", "conf.yaml")
)
