package vm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const disksLocation = "/storage/vm"
const zfsPool string = "storage/vm"

func crash(err error) {
	handleError(err)
}

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

func vnc(port int, fullScreen bool, shouldWait bool) string {
	s := fmt.Sprintf("fbuf,tcp=0.0.0.0:%d", port)
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

// Start starts VM
// TODO move iso into something like attach cdrom
func (vm VM) Start(fullScreen bool, iso *string) {
	numberOfCPUs := strconv.Itoa(runtime.NumCPU())
	memory := "10G"

	//TODO Because of hardcoded LPC 31, have limit to 30 slots
	slots := []string{
		"hostbridge",
		//"lpc",
		networkDevice("tap0"),
		vm.diskSlot(),
		vnc(vm.VNCPort(), fullScreen, false),
		"xhci,tablet",
	}

	if iso != nil {
		slots = append(slots, cdRom(*iso))
	}

	args := append(numberSlots(slots), []string{
		"-s",
		"31:0,lpc",
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

	log.Print(args)

	cmd := exec.Command("bhyve", args...)

	//	cmd.Stdin = os.Stdin
	//	cmd.Stderr = os.Stderr
	//	cmd.Stdout = os.Stdout

	err := cmd.Start()
	handleError(err)

}

//Stop stops vm
func (vm VM) Stop() {
	out, err := exec.Command("bhyvectl", "--vm="+vm.Name, "--destroy").Output()
	if err != nil {
		log.Print(string(out))
	}
}

// VM is vm
type VM struct {
	Name       string
	Referenced string
	Used       string
	index      int // this is to figure out all things like ports etc
}

func (vm VM) diskPath() string {
	return disksLocation + "/" + vm.Name + "/disk"
}

func (vm VM) configurationPath() string {
	return disksLocation + "/" + vm.Name + "/configuration.json"
}

func (vm VM) isRunning() bool {
	_, err := os.Lstat("/dev/vmm/" + vm.Name)
	return err == nil
}

func (vm VM) zfsDataset() string {
	return zfsPool + "/" + vm.Name
}

//VNCPort VNC port
func (vm VM) VNCPort() int {
	return 5900 + vm.index
}

// Create creates vm and all related things such as zfs datasets etc
func (vm VM) Create() {

	err := exec.Command("zfs", "create", vm.zfsDataset()).Run()
	handleError(err)

	//TODO try non file based storage
	err = exec.Command("truncate", "-s", "100G", vm.diskPath()).Run()
	handleError(err)

}

func (vm VM) diskSlot() string {
	//TODO figure out which one is better
	//return "virtio-blk," + vm.diskPath()
	return "ahci-hd," + vm.diskPath()
}

// CloneFrom creates new vm from snapshot of other vm
func (vm VM) CloneFrom(fromSnapshot string) {
	err := exec.Command("zfs", "clone", zfsPool+"/"+fromSnapshot, vm.zfsDataset()).Run()
	handleError(err)
}

// Snapshot takes snapshot a give vm
func (vm VM) Snapshot(name string) {
	now := time.Now()
	snapshotTime := fmt.Sprintf("%d%02d%02d-%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())

	if name != "" {
		snapshotTime = name
	}
	snapshotName := vm.zfsDataset() + "@" + snapshotTime

	log.Printf("Creating snapshot with name %s", snapshotName)
	err := exec.Command("zfs", "snap", snapshotName).Run()
	handleError(err)
}

// New returns new VM stract ready for usage
func New(name string) VM {
	entries, err := filepath.Glob("/dev/vmm/*")
	crash(err)
	return VM{Name: name, index: len(entries)}
}

// List lists all vms and their status
func List() {
	output, err := exec.Command("zfs", "list", "-r", "-H", zfsPool).Output()
	handleError(err)
	lines := strings.Split(string(output), "\n")

	fmt.Println("Name\tRunning\tUsed\tRefer\tVNC Port")

	//TODO panic
	for _, line := range lines[1 : len(lines)-1] {
		//TODO store ref and usage
		datasetName := strings.Split(line, "\t")[0]
		used := strings.Split(line, "\t")[1]
		referenced := strings.Split(line, "\t")[3]
		vmName := strings.Replace(datasetName, zfsPool+"/", "", 1)
		vm := VM{Name: vmName, Referenced: referenced, Used: used}
		fmt.Printf("%s\t%t\t%s\t%s\t%d\n", vmName, vm.isRunning(), vm.Used, vm.Referenced, vm.VNCPort())
	}

}
