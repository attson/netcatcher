package route

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolverHelperScriptSyntax(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/sh", "-n", helper).CombinedOutput(); err != nil {
		t.Fatalf("helper script syntax is invalid: %v: %s", err, out)
	}
}

func TestAuthorizationUpToDate(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	run := func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if !authorizationUpToDate(helper, run) {
		t.Fatal("expected current helper and sudo capabilities to be accepted")
	}
	want := [][]string{
		{"sudo", "-n", "/sbin/route", "-n", "get", "127.0.0.1"},
		{"sudo", "-n", helper, "remove"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected authorization probes: got %v, want %v", calls, want)
	}
}

func TestAuthorizationUpToDateRejectsStaleHelper(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	called := false
	if authorizationUpToDate(helper, func(string, ...string) error {
		called = true
		return nil
	}) {
		t.Fatal("expected stale helper to be rejected")
	}
	if called {
		t.Fatal("sudo probes should not run for a stale helper")
	}
}

func TestAuthorizationUpToDateRequiresEverySudoCapability(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		failingCall int
	}{
		{name: "route", failingCall: 1},
		{name: "helper", failingCall: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			run := func(string, ...string) error {
				calls++
				if calls == test.failingCall {
					return errors.New("not allowed")
				}
				return nil
			}
			if authorizationUpToDate(helper, run) {
				t.Fatalf("expected sudo probe %d failure to invalidate authorization", test.failingCall)
			}
		})
	}
}
