// Copyright 2019 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// macat implements a nanocat(1) work-alike command.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/optopia"

	"go.nanomsg.org/mangos/v3/macat"
)

var args []string
var exitFunc func(int)
var stdErr io.Writer

func init() {
	args = os.Args
	exitFunc = os.Exit
	stdErr = os.Stderr
}

func main() {

	a := &macat.App{}
	a.Initialize()
	e := a.Run(args[1:]...)
	if e != nil {
		_, _ = fmt.Fprintf(stdErr, "%s: %v\n", args[0], e)
		if strings.HasPrefix(e.Error(), "usage:") ||
			optopia.ErrNoSuchOption.Is(e) ||
			optopia.ErrOptionRequiresValue.Is(e) {

			help := a.Help()
			_, _ = fmt.Fprintf(stdErr, "Usage: %s <Options>\n", args[0])
			_, _ = fmt.Fprint(stdErr, help)
			_, _ = fmt.Fprintln(stdErr, "")
		}
		exitFunc(1)
	}
}
