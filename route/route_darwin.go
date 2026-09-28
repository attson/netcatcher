package route

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"

	"netcatcher/llog"
)

const (
	sudoersFile = "/etc/sudoers.d/netcatcher"
	helperPath  = "/usr/local/sbin/netcatcher-resolver-helper"
)

// resolverHelperScript is installed at helperPath during the one-time admin
// auth. It is the only privileged operation NetCatcher invokes besides
// /sbin/route. It strictly validates its arguments so the (broad on paper)
// NOPASSWD sudoers entry cannot be abused to write arbitrary files.
const resolverHelperScript = `#!/bin/sh
set -e
usage() { echo "usage: $0 install <port> <domain>... | remove <domain>... | flush" >&2; exit 2; }
sanitize_domain() {
    case "$1" in
        '' | *[!a-zA-Z0-9.-]* ) return 1 ;;
    esac
    return 0
}
sanitize_port() {
    case "$1" in
        '' | *[!0-9]* ) return 1 ;;
    esac
    [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}
flush_dns() {
    # dscacheutil clears the directory-service cache; notifying
    # mDNSResponder makes newly-installed scoped resolvers take effect for
    # subsequent system lookups immediately.  Either command may be absent
    # on future macOS releases, so cache refresh is intentionally best-effort.
    /usr/bin/dscacheutil -flushcache >/dev/null 2>&1 || true
    /usr/bin/killall -HUP mDNSResponder >/dev/null 2>&1 || true
}
cmd="$1"; shift || usage
case "$cmd" in
install)
    port="$1"; shift || usage
    sanitize_port "$port" || { echo "invalid port: $port" >&2; exit 3; }
    mkdir -p /etc/resolver
    for d in "$@"; do
        sanitize_domain "$d" || { echo "invalid domain: $d" >&2; continue; }
        printf 'nameserver 127.0.0.1\nport %s\n' "$port" > "/etc/resolver/$d"
    done
    flush_dns
    ;;
remove)
    for d in "$@"; do
        sanitize_domain "$d" || { echo "invalid domain: $d" >&2; continue; }
        rm -f "/etc/resolver/$d"
    done
    # "remove" with no domains is also used as the read-only sudo probe.
    [ "$#" -eq 0 ] || flush_dns
    ;;
flush)
    [ "$#" -eq 0 ] || usage
    flush_dns
    ;;
*) usage ;;
esac
`

var authOnce sync.Once

func isRoot() bool {
	return os.Geteuid() == 0
}

// HelperPath exposes the installed helper's absolute path to other packages
// (currently the resolver file manager) without re-declaring the constant.
func HelperPath() string { return helperPath }

func ensureAuth() {
	if isRoot() {
		return
	}
	authOnce.Do(func() {
		if authUpToDate() {
			return
		}
		u, err := user.Current()
		if err != nil {
			llog.Warnf("auth", "get current user failed: %v", err)
			return
		}
		sudoersBody := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: /sbin/route\\n%s ALL=(ALL) NOPASSWD: %s\\n", u.Username, u.Username, helperPath)
		// Escape the helper script for embedding in the shell command below.
		// We write it via base64 → decode to sidestep every shell-quoting
		// gotcha. AppleScript "do shell script" hands the string straight to sh.
		encoded := base64.StdEncoding.EncodeToString([]byte(resolverHelperScript))
		script := fmt.Sprintf(
			`mkdir -p $(dirname %s) && printf %%s '%s' | base64 -d > %s && chown root:wheel %s && chmod 0755 %s && printf '%s' > %s && chmod 0440 %s`,
			helperPath, encoded, helperPath, helperPath, helperPath, sudoersBody, sudoersFile, sudoersFile,
		)
		ascript := fmt.Sprintf(`do shell script "%s" with administrator privileges with prompt "NetCatcher 需要管理员权限以配置系统路由表和 DNS 解析（仅首次运行时需要授权）。"`, script)
		command := exec.Command("osascript", "-e", ascript)
		command.Stderr = log.Writer()
		command.Stdout = log.Writer()
		if err := command.Run(); err != nil {
			llog.Errorf("auth", "admin authorization failed: %v", err)
			return
		}
		if !authUpToDate() {
			llog.Errorf("auth", "admin authorization completed but installed permissions are invalid")
			return
		}
		llog.Infof("auth", "admin authorization granted")
	})
}

type authCommandRunner func(name string, args ...string) error

// authUpToDate verifies the installed helper and both passwordless sudo
// capabilities. Checking only for file existence is insufficient because
// releases before the resolver helper left a valid-looking but incomplete
// sudoers file behind.
func authUpToDate() bool {
	return authorizationUpToDate(helperPath, func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	})
}

func authorizationUpToDate(installedHelper string, run authCommandRunner) bool {
	data, err := os.ReadFile(installedHelper)
	if err != nil || string(data) != resolverHelperScript {
		return false
	}
	st, err := os.Stat(installedHelper)
	if err != nil || st.Mode().Perm() != 0o755 {
		return false
	}

	// Both probes are read-only: route only inspects localhost, while helper
	// remove with no domains performs no filesystem changes.
	if err := run("sudo", "-n", "/sbin/route", "-n", "get", "127.0.0.1"); err != nil {
		return false
	}
	if err := run("sudo", "-n", installedHelper, "remove"); err != nil {
		return false
	}
	return true
}

func runRoute(args ...string) error {
	ensureAuth()
	var command *exec.Cmd
	if isRoot() {
		command = exec.Command("/sbin/route", args...)
	} else {
		command = exec.Command("sudo", append([]string{"/sbin/route"}, args...)...)
	}
	command.Stderr = log.Writer()
	command.Stdout = log.Writer()
	return command.Run()
}

func AddRoute(spec RouteSpec) error {
	args, err := darwinRouteArgs("add", spec)
	if err != nil {
		return err
	}
	return runRoute(args...)
}

func DeleteRoute(spec RouteSpec) error {
	args, err := darwinRouteArgs("delete", spec)
	if err != nil {
		return err
	}
	return runRoute(args...)
}

func darwinRouteArgs(action string, spec RouteSpec) ([]string, error) {
	_, ipv6, err := validateRouteSpec(spec)
	if err != nil {
		return nil, err
	}
	family := "-inet"
	if ipv6 {
		family = "-inet6"
	}
	kind := "-host"
	if spec.Mask != nil {
		kind = "-net"
	}
	args := []string{action, family, kind}
	if spec.InterfaceName != "" {
		args = append(args, "-ifscope", spec.InterfaceName)
	}
	args = append(args, spec.IP)
	if ipv6 && spec.Mask != nil {
		ones, _ := spec.Mask.Size()
		args = append(args, "-prefixlen", strconv.Itoa(ones))
	}
	args = append(args, spec.Gateway)
	if !ipv6 && spec.Mask != nil {
		args = append(args, net.IP(spec.Mask).String())
	}
	return args, nil
}
