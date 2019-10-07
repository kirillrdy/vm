package main

import (
	"flag"
	"github.com/kirillrdy/vm"
	"log"
	"os/exec"
)

//TODO run more than one thing
func main() {

	//Only do if needed
	err := exec.Command("kldload", "-n", "vmm").Run()
	vm.LogError(err, "kldload vmm")

	err = exec.Command("kldload", "-n", "nmdm").Run()
	vm.LogError(err, "kldload nmdm")
	//

	fullScreen := flag.Bool("f", true, "Fullscreen")
	flag.Parse()

	switch flag.Arg(0) {
	case "create":
		vm := vm.New(flag.Arg(1))
		vm.Create()
	case "start":
		//TODO load vmm
		vm := vm.New(flag.Arg(1))
		vm.Start(*fullScreen, nil)

	case "install":
		//TODO load vmm
		vm := vm.New(flag.Arg(1))
		iso := flag.Arg(2)
		vm.Start(*fullScreen, &iso)
	case "stop":
		vm := vm.New(flag.Arg(1))
		vm.Stop()

	case "snap", "snapshot":
		vm := vm.New(flag.Arg(1))
		snapshotName := flag.Arg(2)
		vm.Snapshot(snapshotName)

	case "clone":
		vm := vm.New(flag.Arg(2))
		vm.CloneFrom(flag.Arg(1))
	case "list":
		vm.List()
	case "":
		//TODO
		log.Fatalf("TODO")
	default:
		log.Fatalf("Dont know %s", flag.Arg(0))
	}

}
