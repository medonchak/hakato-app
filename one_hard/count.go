package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run count.go ")
	}
	filePath := os.Args[1]

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}
	text := string(data)
	words := strings.Fields(text)
	lines := strings.Split(text, "\n")

	fmt.Printf("CHaracters : %d\n", len(text))
	fmt.Printf("Words: %d\n", len(words))
	fmt.Printf("Lines: %d\n", len(lines))

}
