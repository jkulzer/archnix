package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"

	yaml "gopkg.in/yaml.v3"

	"github.com/Jguer/go-alpm/v2"
	git "github.com/go-git/go-git/v5"
)

type Package struct {
	Name string `yaml:"package"`
}

type Config struct {
	PackageList
}

type PackageList struct {
	Source           string `yaml:"source"`
	GitConfig        GitConfig
	FilesystemConfig FilesystemConfig
}

type FilesystemConfig struct {
	Path string `yaml:"path"`
}

type GitConfig struct {
	Repository string `yaml:"repository"`
	Path       string `yaml:"path"`
}

func main() {
	enableMultilib := flag.Bool("multilib", false, "enable the multilib repo")
	writeStateFlagSet := flag.NewFlagSet("write-state", flag.ExitOnError)
	overwriteState := writeStateFlagSet.Bool("overwrite", false, "overwrite existing package state file")
	flag.Parse()

	if len(os.Args) < 2 {
		//implement the subcommands actually showing
		fmt.Println("Expected an\n - write\n - diff\n - apply\n - show\n\tsubcommand. Refer to --help for an overview")
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
	case "write":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		packageList := getInstalledPackages(enableMultilib)
		writePackageList(packageList, overwriteState)
	case "diff":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		desiredPackageList := getDesiredPackageList()
		currentPackageList := getInstalledPackages(enableMultilib)
		toInstall, toRemove := diffPackageList(desiredPackageList, currentPackageList)

		printPackageDiff(toInstall, toRemove)
	case "state":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		currentPackageList := getInstalledPackages(enableMultilib)
		showState(currentPackageList)
	case "apply":
		if runningAsRoot == true {
			fmt.Println("Do not run as root")
			os.Exit(1)
		}
		desiredPackageList := getDesiredPackageList()
		currentPackageList := getInstalledPackages(enableMultilib)
		toInstall, toRemove := diffPackageList(desiredPackageList, currentPackageList)
		applyState(toInstall, toRemove)
	case "git":
		getSourceFromGit()
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
	//otherwise you would get all the packages in the Arch Linux repos
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

	var packagesToInstall []Package
	err := yaml.Unmarshal(toInstall, &packagesToInstall)
	if err != nil {
		log.Fatal(err)
	}

	var packagesToInstallPacmanList string

	for _, pkgToRemove := range packagesToInstall {
		packagesToInstallPacmanList = packagesToInstallPacmanList + string(pkgToRemove.Name) + " "
	}

	if packagesToInstallPacmanList != "" {
		pacmanCommandToInstall := "yay -S --noconfirm" + " " + packagesToInstallPacmanList
		fmt.Println("Packages getting installed: " + packagesToInstallPacmanList)
		cmd := exec.Command("bash", "-c", pacmanCommandToInstall)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	} else {
		fmt.Println("No packages getting installed")
	}

	var packagesToRemove []Package
	err = yaml.Unmarshal(toRemove, &packagesToRemove)
	if err != nil {
		log.Fatal(err)
	}

	var packagesToRemovePacmanList string

	for _, pkgToRemove := range packagesToRemove {
		packagesToRemovePacmanList = packagesToRemovePacmanList + string(pkgToRemove.Name) + " "
	}

	if packagesToRemovePacmanList != "" {
		pacmanCommandToRemove := "yay -Rsn --noconfirm" + " " + packagesToRemovePacmanList
		fmt.Println("Packages getting removed: " + packagesToRemovePacmanList)
		cmd := exec.Command("bash", "-c", pacmanCommandToRemove)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	} else {
		fmt.Println("No packages getting removed")
	}
}

func printPackageDiff(toInstall []byte, toRemove []byte) {
	if string(toInstall) != "[]" {
		fmt.Println(
			string(toInstall),
		)
	} else {
		fmt.Println("No packages to install")
	}

	if string(toRemove) != "[]" {
		fmt.Println(
			string(toRemove),
		)
	} else {
		fmt.Println("No packages to remove")
	}
}

func getSourceFromGit() {
	gitUrl := "https://github.com/jkulzer/dotfiles"

	_, err := git.PlainClone(
		"/tmp/archnix/sourceState",
		false,
		&git.CloneOptions{
			URL:   gitUrl,
			Depth: 1,
		},
	)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

}

func parseConfig() {
	if _, err := os.Stat("/etc/archnix"); os.IsNotExist(err) {
		os.Mkdir("/etc/archnix", 0750)
	}

	defaultConfig := `
config:


	`

	if _, err := os.Stat("/etc/archnix/config.yaml"); err == nil {

		os.WriteFile("/etc/archnix/config.yaml", []byte(defaultConfig), 0750)

	}

	//var config []Config
}
