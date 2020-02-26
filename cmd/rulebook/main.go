package main

import (
	"fmt"
	"os"

	"github.com/jloup/ebenaum/rulebook"
)

func main() {
	err := rulebook.Build(os.Stdin, os.Stdout, rulebook.BuilderConfig{true})
	if err != nil {
		fmt.Println(err)
	}
}
