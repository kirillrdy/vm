package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func handleError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

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

func vnc(fullScreen bool, shouldWait bool) string {
	s := "fbuf,tcp=0.0.0.0:5900"
	if fullScreen {
		s += ",w=1920,h=1080"
	} else {
		s += ",w=1280,h=720"
	}

	if shouldWait {
		s += ",wait"
	}
	return s
}

//TODO check dependencies
func uEFIBoot() string {
	return "bootrom,/usr/local/share/uefi-firmware/BHYVE_UEFI.fd"
}

func (vm VM) start(fullScreen bool, iso *string) {
	//TODO maybe give all cpus ?
	numberOfCPUs := "8"
	memory := "10G"

	slots := []string{
		"hostbridge",
		"lpc",
		networkDevice("tap0"),
		vm.diskSlot(),
		vnc(fullScreen, false),
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

	//cmd.Stdin = os.Stdin
	//cmd.Stderr = os.Stderr
	//cmd.Stdout = os.Stdout

	err := cmd.Start()
	handleError(err)

}

func (vm VM) stop() {
	err := exec.Command("bhyvectl", "--vm="+vm.Name, "--destroy").Run()
	handleError(err)
}

// Configuration will contain persisted on disk configuration
type Configuration struct {
	DiskPath string
}

// VM is vm
type VM struct {
	Name string
}

const zfsPool string = "storage/vm"

func (vm VM) diskPath() string {
	return disksLocation + "/" + vm.Name + "/disk"
}

func (vm VM) configurationPath() string {
	return disksLocation + "/" + vm.Name + "/configuration.json"
}

// Create for now only creates disk
func (vm VM) Create() {

	err := exec.Command("zfs", "create", zfsPool+"/"+vm.Name).Run()
	handleError(err)

	err = exec.Command("truncate", "-s", "100G", vm.diskPath()).Run()
	handleError(err)

	configuration := Configuration{DiskPath: vm.diskPath()}
	file, err := os.Create(vm.configurationPath())
	handleError(err)
	defer func() {
		err := file.Close()
		handleError(err)
	}()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(&configuration)
	handleError(err)

}

func (vm VM) diskSlot() string {
	return "virtio-blk," + vm.diskPath()
}

const disksLocation = "/storage/vm"

//TODO run more than one thing
func main() {

	fullScreen := flag.Bool("f", true, "Fullscreen")
	flag.Parse()

	switch flag.Arg(0) {
	case "create":
		vm := VM{Name: flag.Arg(1)}
		vm.Create()
	case "list":
	case "start":

		//TODO load vmm
		vm := VM{Name: flag.Arg(1)}
		vm.start(*fullScreen, nil)

	case "install":
		//TODO load vmm
		vm := VM{Name: flag.Arg(1)}
		iso := flag.Arg(2)
		vm.start(*fullScreen, &iso)
	case "stop":
		vm := VM{Name: flag.Arg(1)}
		vm.stop()
	case "":
		//TODO
		log.Fatalf("TODO")
	default:
		log.Fatalf("Dont know %s", flag.Arg(0))
	}

}
