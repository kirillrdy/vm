package vm

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
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

func (vm VM) storeVNCPort(port int) {
	config := vm.configuration()
	config.VNCPort = port
	vm.writeConfiguration(config)
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

func (vm VM) Start(fullScreen bool, iso *string) {
	numberOfCPUs := strconv.Itoa(runtime.NumCPU())
	memory := "10G"

	configuration := vm.configuration()

	//TODO Because of hardcoded LPC 31, have limit to 30 slots
	slots := []string{
		"hostbridge",
		//"lpc",
		networkDevice("tap0"),
		vm.diskSlot(),
		vnc(configuration.VNCPort, fullScreen, false),
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

// Configuration will contain persisted on disk configuratior
type Configuration struct {
	VNCPort int
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

// Create creates vm and all related things such as zfs datasets etc
func (vm VM) Create() {

	err := exec.Command("zfs", "create", vm.zfsDataset()).Run()
	handleError(err)

	//TODO try non file based storage
	err = exec.Command("truncate", "-s", "100G", vm.diskPath()).Run()
	handleError(err)

	// Or is better to have this dynamic ? Maybe better dynamic
	configuration := Configuration{VNCPort: nextAvailibleVNCPort()}
	vm.writeConfiguration(configuration)
}

func (vm VM) configuration() Configuration {
	file, err := os.Open(vm.configurationPath())
	handleError(err)
	decoder := json.NewDecoder(file)
	var configuration Configuration
	err = decoder.Decode(&configuration)
	handleError(err)
	return configuration
}

func (vm VM) writeConfiguration(configuration Configuration) {
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
	//TODO figure out which one is better
	//return "virtio-blk," + vm.diskPath()
	return "ahci-hd," + vm.diskPath()
}

func (vm VM) CloneFrom(fromSnapshot string) {
	err := exec.Command("zfs", "clone", zfsPool+"/"+fromSnapshot, vm.zfsDataset()).Run()
	handleError(err)
}

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

func nextAvailibleVNCPort() int {
	start := 5900

	for i := start; i < start+100; i++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", i))

		if err == nil {
			err = listener.Close()
			crash(err)
			return i
		}
		log.Print(err)

	}
	log.Panic("Hmm ran out of ports ?")
	return 0
}

// New returns new VM stract ready for usage
func New(name string) VM {
	//TODO guess index
	return VM{Name: name, index: 0}
}

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
		fmt.Printf("%s\t%t\t%s\t%s\t%d\n", vmName, vm.isRunning(), vm.Used, vm.Referenced, vm.configuration().VNCPort)
	}

}
