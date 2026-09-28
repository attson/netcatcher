package route

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func maskString(mask net.IPMask) string {
	str := ""
	for _, b := range mask {
		// byte to int
		str += fmt.Sprintf("%d.", b)
	}

	return str[:len(str)-1]
}

func AddRoute(spec RouteSpec) error {
	args, err := windowsRouteArgs("add", spec)
	if err != nil {
		return err
	}
	return runWindowsRoute(args...)
}

func DeleteRoute(spec RouteSpec) error {
	args, err := windowsRouteArgs("delete", spec)
	if err != nil {
		return err
	}
	return runWindowsRoute(args...)
}

func runWindowsRoute(args ...string) error {
	command := exec.Command("route", args...)
	command.Stderr = NewW(log.Writer())
	command.Stdout = NewW(log.Writer())
	return command.Run()
}

func windowsRouteArgs(action string, spec RouteSpec) ([]string, error) {
	_, ipv6, err := validateRouteSpec(spec)
	if err != nil {
		return nil, err
	}
	if ipv6 {
		prefix := 128
		if spec.Mask != nil {
			prefix, _ = spec.Mask.Size()
		}
		gateway := strings.SplitN(spec.Gateway, "%", 2)[0]
		args := []string{"-6", action, spec.IP + "/" + strconv.Itoa(prefix), gateway}
		if spec.InterfaceIndex > 0 {
			args = append(args, "if", strconv.Itoa(spec.InterfaceIndex))
		}
		return args, nil
	}
	mask := net.CIDRMask(32, 32)
	if spec.Mask != nil {
		mask = spec.Mask
	}
	args := []string{"-4", action, spec.IP, "mask", maskString(mask), spec.Gateway}
	if spec.InterfaceIndex > 0 {
		args = append(args, "if", strconv.Itoa(spec.InterfaceIndex))
	}
	return args, nil
}

type GBKW struct {
	w io.Writer
}

func NewW(w io.Writer) *GBKW {
	return &GBKW{w: w}
}

func (w GBKW) Write(p []byte) (n int, err error) {
	gbk, err := gbkToUtf8(p)
	if err != nil {
		return 0, err
	}

	return w.w.Write(gbk)
}

func gbkToUtf8(s []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	d, e := ioutil.ReadAll(reader)
	if e != nil {
		return nil, e
	}
	return d, nil
}
