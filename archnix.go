package main

import (
	"flag"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"

	"github.com/Jguer/go-alpm/v2"
	"github.com/google/go-cmp/cmp"
)

type Package struct {
	Name    string `yaml:"packageName"`
	Version string `yaml:"packageVersion"`
}

func main() {
	enableMultilib := flag.Bool("multilib", false, "enable the multilib repo")
	writeStateFlagSet := flag.NewFlagSet("write-state", flag.ExitOnError)
	overwriteState := writeStateFlagSet.Bool("overwrite", false, "overwrite existing package state file")
	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Println("Expected an argument. Refer to --help for an overview")
		os.Exit(1)
	} else {
		writeStateFlagSet.Parse(os.Args[2:])
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

func writePackageList(packageList []byte, overwriteState *bool) {
	if _, err := os.Stat("/var/lib/archnix"); os.IsNotExist(err) {
		os.Mkdir("/var/lib/archnix", 0750)
	}

	if _, err := os.Stat("/var/lib/archnix/packages.yaml"); err == nil {

		if *overwriteState {
			os.WriteFile("/var/lib/archnix/packages.yaml", packageList, 0750)
		} else {
			fmt.Println("A state file exists. \nPass -overwrite to overwrite the current state.")
		}

	} else {
		os.WriteFile("/var/lib/archnix/packages.yaml", packageList, 0750)
	}

}

func getDesiredPackageList() (desiredPackageList []byte) {
	readPackageList, err := os.ReadFile("/var/lib/archnix/packages.yaml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	desiredPackageList = readPackageList
	return desiredPackageList
}

func diffPackageList(desiredPackageList []byte, currentPackageList []byte) {

	if !cmp.Equal(desiredPackageList, currentPackageList) {
		fmt.Println(cmp.Diff(currentPackageList, desiredPackageList))
	} else {
		fmt.Println("State up to date")
	}
}

func getInstalledPackages(enableMultilib *bool) (installedPackages []byte) {

	//this initializes alpm
	h, err := alpm.Initialize("/", "/var/lib/pacman")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	defer h.Release()

	//defines the db as the local one where pacman stores its (installed) packages
	db, err := h.LocalDB()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var allPackagesYaml []Package

	//runs through every package present in the database (means is installed)
	for _, pkg := range db.PkgCache().Slice() {

		// pkg.Reason is 0 if the package was explicitly installed (not as a dependency)
		// this means that only packages which were explicity installed get added to the list
		//(dependencies will get automatically installed and orphans automatically removed)
		if pkg.Reason() == 0 {

			singlePackageYAML := Package{
				Name:    pkg.Name(),
				Version: pkg.Version(),
			}

			allPackagesYaml = append(allPackagesYaml, singlePackageYAML)
		}
	}
	packageData, err := yaml.Marshal(allPackagesYaml)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	return packageData
}
