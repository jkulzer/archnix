package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	yaml "gopkg.in/yaml.v3"

	"github.com/Jguer/go-alpm/v2"
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
	case "show-state":
		currentPackageList := getInstalledPackages(enableMultilib)
		showState(currentPackageList)
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

type PackageChange struct {
	Name            string `yaml:"packageName"`
	PreviousVersion string `yaml:"previousVersion"`
	NewVersion      string `yaml:"newVersion"`
}

func diffPackageList(desiredPackageList []byte, currentPackageList []byte) {

	var currentPackagesStruct, desiredPackagesStruct []Package

	err := yaml.Unmarshal(currentPackageList, &currentPackagesStruct)
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(desiredPackageList, &desiredPackagesStruct)
	if err != nil {
		log.Fatal(err)
	}

	var removals []Package
	var additions []Package
	var changes []PackageChange

	for _, desiredPkg := range desiredPackagesStruct {
		found := false
		for _, currentPkg := range currentPackagesStruct {
			if desiredPkg.Name == currentPkg.Name {
				found = true
				if desiredPkg.Version != currentPkg.Version {
					changes = append(changes, PackageChange{
						Name:            desiredPkg.Name,
						PreviousVersion: currentPkg.Version,
						NewVersion:      desiredPkg.Version,
					})
				}
				break
			}
		}
		if !found {
			additions = append(additions, desiredPkg)
		}
	}

	for _, currentPkg := range currentPackagesStruct {
		found := false
		for _, desiredPkg := range desiredPackagesStruct {
			if desiredPkg.Name == currentPkg.Name {
				found = true
				break
			}
		}
		if !found {
			removals = append(removals, currentPkg)
		}
	}

	removalsOutput, err := yaml.Marshal(&removals)
	if err != nil {
		log.Fatal(err)
	}

	additionsOutput, err := yaml.Marshal(&additions)
	if err != nil {
		log.Fatal(err)
	}

	changesOutput, err := yaml.Marshal(&changes)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Removals:")
	fmt.Println(string(removalsOutput))

	fmt.Println("Additions:")
	fmt.Println(string(additionsOutput))

	fmt.Println("Changes:")
	fmt.Println(string(changesOutput))
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

func showState(currentPackageList []byte) {
	fmt.Println(
		fmt.Sprintf(string(currentPackageList)),
	)
}
