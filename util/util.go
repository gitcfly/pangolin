package util

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ParseAddr(addr string) (ip string, port int) {
	as := strings.Split(addr, ":")
	ip = as[0]
	port, _ = strconv.Atoi(as[1])
	return
}

func ExcCmd(cmd string) {
	err := exec.Command("bash", "-c", cmd).Run()
	if err != nil {
		fmt.Println(err)
	}
}
