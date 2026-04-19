package route

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
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
usage() { echo "usage: $0 install <port> <domain>... | remove <domain>..." >&2; exit 2; }
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
    ;;
remove)
    for d in "$@"; do
        sanitize_domain "$d" || { echo "invalid domain: $d" >&2; continue; }
        rm -f "/etc/resolver/$d"
    done
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
		llog.Infof("auth", "admin authorization granted")
	})
}

// authUpToDate returns true when both the sudoers file and the helper script
// already exist — meaning a previous session has already installed them.
func authUpToDate() bool {
	if _, err := os.Stat(sudoersFile); err != nil {
		return false
	}
	st, err := os.Stat(helperPath)
	if err != nil {
		return false
	}
	if st.Size() == 0 {
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

func AddRoute(ip, gateway string, mask net.IPMask) error {
	if mask != nil {
		return runRoute("add", "-net", ip, gateway)
	}
	return runRoute("add", "-host", ip, gateway)
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	if mask != nil {
		return runRoute("delete", "-net", ip, gateway)
	}
	return runRoute("delete", "-host", ip, gateway)
}

type RouteSpec struct {
	Ip      string
	Gateway string
	Mask    net.IPMask
}

func AddRoutes(routes []RouteSpec) error {
	for _, r := range routes {
		if err := AddRoute(r.Ip, r.Gateway, r.Mask); err != nil {
			llog.Warnf("route", "add %s failed: %v", r.Ip, err)
		}
	}
	return nil
}

func DeleteRoutes(routes []RouteSpec) error {
	for _, r := range routes {
		if err := DeleteRoute(r.Ip, r.Gateway, r.Mask); err != nil {
			llog.Warnf("route", "delete %s failed: %v", r.Ip, err)
		}
	}
	return nil
}
