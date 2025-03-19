package main

import (
	"fmt"

	"github.com/tdewolff/argp"
)

func main() {
	var verbose int
	var input string
	var output string
	var files []string
	size := 512 // default value

	cmd := argp.New("CLI tool description")
	cmd.AddOpt(argp.Count{&verbose}, "v", "verbose", "Increase verbosity, eg. -vvv")
	cmd.AddOpt(&output, "o", "output", "Output file name")
	cmd.AddOpt(&size, "", "size", "Image size")
	cmd.AddArg(&input, "input", "Input file name")
	cmd.AddRest(&files, "files", "Additional files")
	cmd.Parse()

	fmt.Printf("verbose: %v\n", verbose)
	fmt.Printf("input: %q\n", input)
	fmt.Printf("output: %q\n", output)
	fmt.Printf("size: %v\n", size)
	fmt.Printf("files: []string{")
	for i, file := range files {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%q", file)
	}
	fmt.Println("}")
}
