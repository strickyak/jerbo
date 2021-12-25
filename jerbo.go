package main

import (
	"log"
	"os"
	"strings"

	"github.com/strickyak/jerbo/v2/os9defs"
	"github.com/strickyak/jerbo/v2/undisk"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("| ")

	command := ""
	args := os.Args
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		command = args[1]
		os.Args = append([]string{args[0]}, args[2:]...)
	}

	commandMap := map[string]func(){
		"os9defs": func() { os9defs.Main() },
		"undisk": func() { undisk.Main() },
	}

	if fn, ok := commandMap[command]; ok {
		fn()
	} else {
		log.Printf("Usage:   go run jerbo.go command -flags...")
		log.Printf("For flags help:   go run jerbo.go command -help")
		log.Printf("Available commands:")
		for k := range commandMap {
			log.Printf("    %q", k)
		}
		log.Fatalf("jerboa: Unknown command: %q", command)
	}
}
