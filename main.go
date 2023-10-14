package main

import (
	"log"

	"github.com/thank243/zteOnu/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Panicln(err)
	}
}
