package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func networkDevice(device string) string {
	return "virtio-net," + device
}

func disk() string {
	return "virtio-blk,./disk.img"
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

func vnc() string {
	return "fbuf,tcp=0.0.0.0:5900,w=1280,h=720,wait"
}

func uEFIBoot() string {
	return "bootrom,/usr/local/share/uefi-firmware/BHYVE_UEFI.fd"
}

func startVM(vmName string, install bool) {
	numberOfCPUs := "4"
	memory := "4G"
	iso := "./install.iso"

	slots := []string{
		"hostbridge",
		"lpc",
		networkDevice("tap0"),
		disk(),
		vnc(),
		"xhci,tablet",
	}

	if install == true {
		slots = append(slots, cdRom(iso))
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
		vmName,
	}...)

	cmd := exec.Command("bhyve", args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

}

func main() {
	startVM("ubuntu", false)
}
