// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildutil_test

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/packages/packagestest"
)

func TestContainingPackage(t *testing.T) {
	if runtime.Compiler == "gccgo" {
		t.Skip("gccgo has no GOROOT")
	}

	exported := packagestest.Export(t, packagestest.GOPATH, []packagestest.Module{
		{Name: "golang.org/x/tools/go/buildutil", Files: packagestest.MustCopyFileTree(".")}})
	defer exported.Cleanup()

	goroot := runtime.GOROOT()
	var gopath string
	for _, env := range exported.Config.Env {
		if !strings.HasPrefix(env, "GOPATH=") {
			continue
		}
		gopath = strings.TrimPrefix(env, "GOPATH=")
	}
	if gopath == "" {
		t.Fatal("Failed to fish GOPATH out of env: ", exported.Config.Env)
	}
	buildutildir := filepath.Join(gopath, "golang.org", "x", "tools", "go", "buildutil")

	type Test struct {
		gopath, filename, wantPkg string
	}

	tests := []Test{
		{gopath, goroot + "/src/fmt/print.go", "fmt"},
		{gopath, goroot + "/src/encoding/json/foo.go", "encoding/json"},
		{gopath, goroot + "/src/encoding/missing/foo.go", "(not found)"},
		{gopath, gopath + "/src/golang.org/x/tools/go/buildutil/util_test.go",
			"golang.org/x/tools/go/buildutil"},
	}

	if runtime.GOOS != "windows" && runtime.GOOS != "plan9" {
		// Make a symlink to gopath for test
		tmp, err := ioutil.TempDir(os.TempDir(), "go")
		if err != nil {
			t.Errorf("Unable to create a temporary directory in %s", os.TempDir())
		}

		defer os.RemoveAll(tmp)

		// symlink between $GOPATH/src and /tmp/go/src
		// in order to test all possible symlink cases
		if err := os.Symlink(gopath+"/src", tmp+"/src"); err != nil {
			t.Fatal(err)
		}
		tests = append(tests, []Test{
			{gopath, tmp + "/src/golang.org/x/tools/go/buildutil/util_test.go", "golang.org/x/tools/go/buildutil"},
			{tmp, gopath + "/src/golang.org/x/tools/go/buildutil/util_test.go", "golang.org/x/tools/go/buildutil"},
			{tmp, tmp + "/src/golang.org/x/tools/go/buildutil/util_test.go", "golang.org/x/tools/go/buildutil"},
		}...)
	}

	for _, test := range tests {
		var got string
		var buildContext = build.Default
		buildContext.GOPATH = test.gopath
		bp, err := buildutil.ContainingPackage(&buildContext, buildutildir, test.filename)
		if err != nil {
			got = "(not found)"
		} else {
			got = bp.ImportPath
		}
		if got != test.wantPkg {
			t.Errorf("ContainingPackage(%q) = %s, want %s", test.filename, got, test.wantPkg)
		}
	}

}

func TestContainingPackageInGoModule(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "buildutil-containing-package-gomod")
	if err != nil {
		t.Errorf("unable to create a temporary directory in %s", os.TempDir())
	}
	defer os.RemoveAll(tmp)

	moduleName := "golang.org/examples/hello"
	gomodFile := filepath.Join(tmp, "go.mod")

	err = ioutil.WriteFile(gomodFile, []byte(fmt.Sprintf("module %s\n", moduleName)), 0644)
	if err != nil {
		t.Errorf("failed to write a go.mod in tmpdir %s: %v", tmp, err)
	}

	if err := os.Mkdir(filepath.Join(tmp, "internal"), 0755); err != nil {
		t.Errorf("failed to create internal dir in tmpdir %s: %v", tmp, err)
	}

	type test struct {
		dir      string
		filename string
	}

	testCases := make([]test, 0)

	// create test files
	for _, file := range []string{
		filepath.Join(tmp, "hello.go"),
		filepath.Join(tmp, "internal", "hello.go"),
	} {
		err := ioutil.WriteFile(
			file,
			[]byte(`
				package hello

				func Hello() string {
					return "Hello, world."
				}
			`),
			0644,
		)
		if err != nil {
			t.Errorf("failed to create hello.go in tmpdir %s: %v", tmp, err)
		}

		testCases = append(testCases, test{tmp, file})
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("SourceFile_%s", tc.filename), func(t *testing.T) {
			buildContext := build.Default
			bp, err := buildutil.ContainingPackage(&buildContext, tc.dir, tc.filename)
			if err != nil {
				t.Errorf("failed to find a package: %v", err)
			}
			if bp.ImportPath != moduleName {
				t.Errorf("invalid ImportPath, got %s, expected %s", bp.ImportPath, moduleName)
			}
		})
	}
}
