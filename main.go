package main

import (
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
	"os"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "forward" {
		common.Forward(os.Stdin, os.Stdout, "")
	} else {
		common.Listen(handler.New(&handler.Opts{}), "", nil)
	}
}
