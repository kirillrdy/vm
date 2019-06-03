package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
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
func uEFIBoot(legacy bool) string {
	if legacy {
		return "bootrom,/usr/local/share/uefi-firmware/BHYVE_UEFI_CSM.fd"
	}
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
		uEFIBoot(false),
		"-l",
		"com1,/dev/nmdm0A", //TODO 0A is hardcoded
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

func (vm VM) zfsDataset() string {
	return zfsPool + "/" + vm.Name
}

// Create for now only creates disk
func (vm VM) Create() {

	err := exec.Command("zfs", "create", vm.zfsDataset()).Run()
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

func (vm VM) cloneFrom(fromSnapshot string) {
	err := exec.Command("zfs", "clone", zfsPool+"/"+fromSnapshot, vm.zfsDataset()).Run()
	handleError(err)
}

func (vm VM) snapshot() {
	now := time.Now()
	snapshotTime := fmt.Sprintf("%d%02d%02d-%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	snapshotName := vm.zfsDataset() + "@" + snapshotTime

	log.Printf("Creating snapshot with name %s", snapshotName)
	err := exec.Command("zfs", "snap", snapshotName).Run()
	handleError(err)
}

const disksLocation = "/storage/vm"

func list() {
	output, err := exec.Command("zfs", "list", "-r", "-H", zfsPool).Output()
	handleError(err)
	lines := strings.Split(string(output), "\n")

	//TODO panic
	for _, line := range lines[1 : len(lines)-1] {
		//TODO store ref and usage
		datasetName := strings.Split(line, "\t")[0]
		vmName := strings.Replace(datasetName, zfsPool+"/", "", 1)
		fmt.Println(vmName)
	}

}

//TODO run more than one thing
func main() {

	fullScreen := flag.Bool("f", true, "Fullscreen")
	flag.Parse()

	switch flag.Arg(0) {
	case "create":
		vm := VM{Name: flag.Arg(1)}
		vm.Create()
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

	case "snap", "snapshot":
		vm := VM{Name: flag.Arg(1)}
		vm.snapshot()

	case "clone":
		vm := VM{Name: flag.Arg(2)}
		vm.cloneFrom(flag.Arg(1))
	case "list":
		list()
	case "":
		//TODO
		log.Fatalf("TODO")
	default:
		log.Fatalf("Dont know %s", flag.Arg(0))
	}

}
