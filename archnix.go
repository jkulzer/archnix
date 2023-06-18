package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Jguer/go-alpm/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
	"os"
)

type Package struct {
	Name    string `json:"packageName"`
	Version string `json:"packageVersion"`
}

func main() {
	enableMultilib := flag.Bool("multilib", false, "enable the multilib repo")
	writeStateFlagSet := flag.NewFlagSet("write-state", flag.ExitOnError)
	overwriteState := writeStateFlagSet.Bool("overwrite", false, "overwrite existing package state file")
	writeStateFlagSet.Parse(os.Args[2:])
	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Println("Expected an argument. Refer to --help for an overview")
		os.Exit(1)
	}

	switch os.Args[1] {

	case "write-state":
		packageList := getInstalledPackages(enableMultilib)
		writePackageList(packageList, overwriteState)
	case "diff-state":
		desiredPackageList := getDesiredPackageList()
		currentPackageList := getInstalledPackages(enableMultilib)
		diffPackageList(desiredPackageList, currentPackageList)
	default:
		fmt.Println(os.Args[1] + " is an invalid argument. Refer to --help for instructions")
		os.Exit(1)
	}

}

func writePackageList(packageList string, overwriteState *bool) {
	if _, err := os.Stat("/var/lib/archnix"); os.IsNotExist(err) {
		os.Mkdir("/var/lib/archnix", 0750)
	}

	if _, err := os.Stat("/var/lib/archnix/packages.json"); err == nil {

		if *overwriteState {
			os.WriteFile("/var/lib/archnix/packages.json", []byte(packageList), 0750)
		} else {
			fmt.Println("A state file exists. \nPass -overwrite-state to overwrite the current state.")
		}

	} else {
		os.WriteFile("/var/lib/archnix/packages.json", []byte(packageList), 0750)
	}

}

func getDesiredPackageList() (desiredPackageList string) {
	readPackageList, err := os.ReadFile("/var/lib/archnix/packages.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	desiredPackageList = string(readPackageList)
	return desiredPackageList
}

func diffPackageList(desiredPackageList string, currentPackageList string) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(desiredPackageList, currentPackageList, true)
	fmt.Println(dmp.DiffToDelta(diffs))
}

func getInstalledPackages(enableMultilib *bool) (installedPackages string) {

	h, err := alpm.Initialize("/", "/var/lib/pacman")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	defer h.Release()

	db, _ := h.RegisterSyncDB("core", 0)
	h.RegisterSyncDB("community", 0)
	h.RegisterSyncDB("extra", 0)
	h.RegisterSyncDB("extra", 0)
	if *enableMultilib {
		h.RegisterSyncDB("multilib", 0)
	}

	var allPackagesJSON []Package

	for _, pkg := range db.PkgCache().Slice() {

		singlePackageJSON := Package{
			Name:    pkg.Name(),
			Version: pkg.Version(),
		}

		allPackagesJSON = append(allPackagesJSON, singlePackageJSON)
	}
	packageData, err := json.MarshalIndent(allPackagesJSON, "", "\t")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	installedPackages = string(packageData)
	return installedPackages
}
