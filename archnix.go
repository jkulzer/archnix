package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"

	yaml "gopkg.in/yaml.v3"

	"github.com/Jguer/go-alpm/v2"
)

type Package struct {
	Name string `yaml:"packageName"`
}

func main() {
	enableMultilib := flag.Bool("multilib", false, "enable the multilib repo")
	writeStateFlagSet := flag.NewFlagSet("write-state", flag.ExitOnError)
	overwriteState := writeStateFlagSet.Bool("overwrite", false, "overwrite existing package state file")
	flag.Parse()

	if len(os.Args) < 2 {
		//implement the subcommands actually showing
		fmt.Println("Expected an\n - write-state\n - diff-state\n - apply-state\n - show-state\n\tsubcommand. Refer to --help for an overview")
		os.Exit(1)
	} else {
		writeStateFlagSet.Parse(os.Args[2:])
	}

	//this whole thing is needed so the program doesn't run as root (causes problems with yay and makepkg)
	currentUser, err := user.Current()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var runningAsRoot bool
	if currentUser.Uid == "0" {
		runningAsRoot = true
	}

	//TODO
	//feature is disabled for testing
	runningAsRoot = false

	switch os.Args[1] {
	case "write-state":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		packageList := getInstalledPackages(enableMultilib)
		writePackageList(packageList, overwriteState)
	case "diff-state":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		desiredPackageList := getDesiredPackageList()
		currentPackageList := getInstalledPackages(enableMultilib)
		toInstall, toRemove := diffPackageList(desiredPackageList, currentPackageList)
		fmt.Println("Packages to Install:")
		fmt.Println(string(toInstall))
		fmt.Println("Packages to Remove:")
		fmt.Println(string(toRemove))
	case "show-state":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		currentPackageList := getInstalledPackages(enableMultilib)
		showState(currentPackageList)
	case "apply-state":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		desiredPackageList := getDesiredPackageList()
		currentPackageList := getInstalledPackages(enableMultilib)
		toInstall, toRemove := diffPackageList(desiredPackageList, currentPackageList)
		applyState(toInstall, toRemove)
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

func diffPackageList(desiredPackageList []byte, currentPackageList []byte) (packagesToAdd []byte, packagesToRemove []byte) {

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

	//this goes through every package in the desired state
	for _, desiredPkg := range desiredPackagesStruct {
		//sets found to false per default
		found := false
		//this runs once for every package in the desired package list
		//if the name of the desired package of the root loop matches any one of the ones in the current packages, the loop exits
		for _, currentPkg := range currentPackagesStruct {
			if desiredPkg.Name == currentPkg.Name {
				found = true
				break
			}
		}
		//if no package matches (the loop exits without any package being found),
		//the found := false carries over and triggers the append below
		if !found {
			additions = append(additions, desiredPkg)
		}
	}

	//thesame steps as above get repeated for packages getting removed
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

	return additionsOutput, removalsOutput
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
				Name: pkg.Name(),
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

func applyState(toInstall []byte, toRemove []byte) {
}
