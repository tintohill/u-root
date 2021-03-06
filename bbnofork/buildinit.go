// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
	"path"
)

func buildinit() {
	e := os.Environ()
	for i := range e {
		if e[i][0:6] == "GOPATH" {
			e[i] = e[i] + ":" + path.Join(config.Uroot, "src/bb/bbsh")
		}
	}
	e = append(e, "CGO_ENABLED=0")
	cmd := exec.Command("go", "build", "-o", "init", ".")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = path.Join(config.Uroot, "src/bb/bbsh")
	cmd.Env = e

	err := cmd.Run()
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
}
