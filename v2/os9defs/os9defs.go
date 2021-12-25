/*
	Scrape important Os9 assembly symbols from a -symbols .list file and generate Go code with const definitions.

	Suggestions:

		go run jerbo.go os9defs -errmsg /home/strick/go/src/github.com/strickyak/doing_os9/gomar/sym/errmsg.txt -listing ../doing_os9/gomar/sym/level2_coco3.list -tag=level2 | gofmt > gomar/sym/level2_defs.go

*/
package os9defs

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// We pick symbols with a single `$` or `.`.
var SymbolMatch = regexp.MustCompile(`^[[].G[]] ([A-Za-z]+)([.$])([A-Za-z0-9]*) +([0-9A-F]{4})\s*$`)
var ErrMsgMatch = regexp.MustCompile(`^([0-9]+) [-]+ (.*)$`)

func SortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func os9defs() {
	if tagFlag != "" {
		fmt.Printf("// +build %s\n", tagFlag)
	}

	// Blank line required between +build directives and package comment.
	fmt.Printf("\n")
	// Avoid the following sentance appearing plain in this source file!
	fmt.Printf("// This file was %s by %s\n", "generated", filepath.Base(os.Args[0]))
	fmt.Printf("package sym\n")
	fmt.Printf("const (\n")

	structs := make(map[string]map[uint]string)
	structs["D."] = make(map[uint]string)
	structs["P$"] = make(map[uint]string)
	structs["PD."] = make(map[uint]string)

	consts := make(map[string]string)
	syscalls := make(map[string]string)
	errnos := make(map[string]string)

	fd, err := os.Open(listingFlag)
	if err != nil {
		log.Fatalf("Cannot open listing file %q: %v", listingFlag, err)
	}
	r := bufio.NewScanner(fd)
	for r.Scan() {
		t := r.Text()
		m := SymbolMatch.FindStringSubmatch(t)
		if m != nil {
			symbol := fmt.Sprintf("%s%s%s", m[1], m[2], m[3])
			constName := fmt.Sprintf("%s_%s", m[1], m[3])
			consts[constName] = m[4]
			fmt.Printf("\t%-12s = 0x%s // %s\n", constName, m[4], symbol)
			if m[2] == "$" && (m[1] == "F" || m[1] == "I") {
				syscalls[symbol] = constName
			}
			if m[2] == "$" && (m[1] == "E") {
				errnos[constName] = m[3]
			}
			value64, _ := strconv.ParseUint(m[4], 16, 16)
			value := uint(value64)
			if d, ok := structs[m[1]+m[2]]; ok {
				d[value] = m[3]
			}
		}
	}
	fmt.Printf(")\n")

	fmt.Printf("var SysCallNames = map[byte]string {\n")
	for _, symbol := range SortedKeys(syscalls) {
		constName := syscalls[symbol]
		fmt.Printf("\t%s: %q,\n", constName, symbol)
	}
	fmt.Printf("}\n")

	fmt.Printf("type Slot struct { off uint; symbol string }\n")
	for s, d := range structs {
		fmt.Printf("var Slots_%s = []Slot {\n", s[:len(s)-1])
		for i := uint(0); i < 512; i++ {
			if symbol, ok := d[i]; ok {
				fmt.Printf("\t{0x%04x, %q},\n", i, symbol)
			}
		}
		fmt.Printf("}\n")
	}

	done := make(map[string]bool)
	fmt.Printf("var Os9Error = map[byte]string {\n")
	for _, errno := range SortedKeys(errnos) {
		val := consts[errno]
		did, _ := done[val]
		if !did {
			fmt.Printf("\t%s: %q,\n", errno, "E$"+errnos[errno])
			done[val] = true
		}
	}
	fmt.Printf("}\n")

	fmt.Printf("var Os9ErrorName = map[byte]string {\n")
	fd, err = os.Open(errmsgFlag)
	if err != nil {
		log.Fatalf("Cannot open errmsg file %q: %v", errmsgFlag, err)
	}
	r = bufio.NewScanner(fd)
	for r.Scan() {
		t := r.Text()
		m := ErrMsgMatch.FindStringSubmatch(t)
		if m == nil {
			log.Fatalf("Cannot ErrMsgMatch this line from %q: %q", errmsgFlag, t)
		}
		fmt.Printf("\t%s: %q,\n", m[1], m[2])
	}
	fmt.Printf("}\n")
}

var listingFlag string
var errmsgFlag string
var tagFlag string

func Main() {
	flags := flag.NewFlagSet("os9defs", 0)
	flags.StringVar(&listingFlag, "listing", "", "assembler listing from lwasm --list=..., with definitions, to read.")
	flags.StringVar(&errmsgFlag, "errmsg", "", "errmsg file from SYS directory, to read.")
	flags.StringVar(&tagFlag, "tag", "", "Mark the output to require this tag, e.g. -tag=level1 or -tag=level2")
	flags.Parse(os.Args[1:])

	os9defs()
}
