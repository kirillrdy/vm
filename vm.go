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

// Crash if there is error
func Crash(err error) {
	if err != nil {
		log.Panic(err)
	}
}

// LogError if there is an error
func LogError(err error, context string) {
	if err != nil {
		log.Printf("ERROR %s: %s", context, err)
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

func (vm VM) tapDevice() string {
	return fmt.Sprintf("tap%d", vm.index)
}
func (vm VM) serialDevice() string {
	return fmt.Sprintf("com1,/dev/nmdm%dA", vm.index)
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
		networkDevice(vm.tapDevice()),
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
		vm.serialDevice(),
		vm.Name,
	}...)

	log.Print(args)

	cmd := exec.Command("bhyve", args...)

	//TODO add some sort of debug of verbose
	//	cmd.Stdin = os.Stdin
	//	cmd.Stderr = os.Stderr
	//	cmd.Stdout = os.Stdout

	//	//Tap 0 is sub optimal
	//	exec.Command("ifconfig", "tap0", "create").Run()
	//	//handleError(err)
	//	exec.Command("ifconfig", "tap0", "up").Run()
	//	//handleError(err)
	//	exec.Command("ifconfig", "bridge0", "create").Run()
	//	//handleError(err)
	//	exec.Command("ifconfig", "bridge0", "addm", "wlan0", "addm", "tap0").Run()
	//	//handleError(err)
	//	exec.Command("ifconfig", "bridge0", "up").Run()
	//	//handleError(err)

	err := cmd.Start()
	Crash(err)

	//TODO localhost
	vnvViewer := exec.Command("vncviewer", fmt.Sprintf("localhost:%d", vm.VNCPort()))
	err = vnvViewer.Start()
	Crash(err)

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
	Crash(err)

	//TODO try non file based storage
	err = exec.Command("truncate", "-s", "100G", vm.diskPath()).Run()
	Crash(err)

}

func (vm VM) diskSlot() string {
	//TODO figure out which one is better
	//return "virtio-blk," + vm.diskPath()
	return "ahci-hd," + vm.diskPath()
}

// CloneFrom creates new vm from snapshot of other vm
func (vm VM) CloneFrom(fromSnapshot string) {
	err := exec.Command("zfs", "clone", zfsPool+"/"+fromSnapshot, vm.zfsDataset()).Run()
	Crash(err)
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
	Crash(err)
}

// New returns new VM stract ready for usage
func New(name string) VM {
	entries, err := filepath.Glob("/dev/vmm/*")
	Crash(err)
	return VM{Name: name, index: len(entries)}
}

// List lists all vms and their status
func List() {
	output, err := exec.Command("zfs", "list", "-r", "-H", zfsPool).Output()
	Crash(err)
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
