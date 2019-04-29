package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func networkDevice(device string) string {
	return "virtio-net," + device
}

func cdRom(iso string) string {
	return "ahci-cd," + iso
}

func numberSlots(slots []string) []string {
	var numberedSlots []string
	for i, slot := range slots {
		numberedSlots = append(numberedSlots, "-s")
		numberedSlots = append(numberedSlots, fmt.Sprintf("%d:0,%s", i, slot))
	}
	return numberedSlots
}

func vnc(shouldWait bool) string {
	s := "fbuf,tcp=0.0.0.0:5900,w=1280,h=720"
	if shouldWait {
		s += ",wait"
	}
	return s
}

func uEFIBoot() string {
	return "bootrom,/usr/local/share/uefi-firmware/BHYVE_UEFI.fd"
}

func (vm VM) start(iso *string) {
	//TODO maybe give all cpus ?
	numberOfCPUs := "8"
	memory := "10G"

	slots := []string{
		"hostbridge",
		"lpc",
		networkDevice("tap0"),
		vm.diskSlot(),
		vnc(false),
		"xhci,tablet",
	}

	if iso != nil {
		slots = append(slots, cdRom(*iso))
	}

	args := append(numberSlots(slots), []string{
		"-AHP",
		"-c",
		numberOfCPUs,
		"-m",
		memory,
		"-l",
		uEFIBoot(),
		"-l",
		"com1,stdio",
		vm.Name,
	}...)

	cmd := exec.Command("bhyve", args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	if err != nil {
		log.Panic(err)
	}

}

// VM is vm
type VM struct {
	Name string
}

func (vm VM) diskPath() string {
	return disksLocation + "/" + vm.Name
}

// Create for now only creates disk
func (vm VM) Create() {
	err := exec.Command("truncate", "-s", "100G", vm.diskPath()).Run()
	if err != nil {
		log.Fatal(err)
	}
}

func (vm VM) diskSlot() string {
	return "virtio-blk," + vm.diskPath()
}

const disksLocation = "/storage/vm"

//TODO run more than one thing
func main() {

	flag.Parse()

	switch flag.Arg(0) {
	case "create":
		vm := VM{Name: flag.Arg(1)}
		vm.Create()
	case "list":
	case "start":

		//TODO load vmm
		vm := VM{Name: flag.Arg(1)}
		vm.start(nil)

	case "install":
		//TODO load vmm
		vm := VM{Name: flag.Arg(1)}
		iso := flag.Arg(2)
		vm.start(&iso)
	case "":
		//TODO
		log.Fatalf("TODO")
	default:
		log.Fatalf("Dont know %s", flag.Arg(0))
	}

}
