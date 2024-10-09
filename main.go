package main

import (
	"log"

	"github.com/stich86/zteOnu/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Panicln(err)
	}
}
