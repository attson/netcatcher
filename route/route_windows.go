package route

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
)

func maskString(mask net.IPMask) string {
	str := ""
	for _, b := range mask {
		// byte to int
		str += fmt.Sprintf("%d.", b)
	}

	return str[:len(str)-1]
}

func AddRoute(ip, gateway string, mask net.IPMask) error {
	var command *exec.Cmd

	if mask != nil {
		command = exec.Command("route", "add", ip, "mask", maskString(mask), gateway)
	} else {
		command = exec.Command("route", "add", ip, "mask", "255.255.255.255", gateway)
	}

	command.Stderr = NewW(os.Stderr)
	command.Stdout = NewW(os.Stdout)

	return command.Run()
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	var command *exec.Cmd

	if mask != nil {
		command = exec.Command("route", "delete", ip, "mask", maskString(mask))
	} else {
		command = exec.Command("route", "delete", ip)
	}

	command.Stderr = NewW(os.Stderr)
	command.Stdout = NewW(os.Stdout)

	return command.Run()
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
