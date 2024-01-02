package main

import (
	"fmt"
	"os"
)

func main() {
	vm := NewVM()
	if vm.LoadProgramFromFile(os.Args[1]) != nil {
		fmt.Println("Error loading program")
		os.Exit(1)
	}
	vm.Run()
}
