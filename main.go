package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func networkDevice() string {
	return "virtio-net,tap0"
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
	return "fbuf,tcp=0.0.0.0:5900,w=800,h=600,wait"
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
		networkDevice(),
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
