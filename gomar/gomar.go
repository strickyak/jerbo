// 6809 & OS-9 & NitrOS-9 Emulator "GOMAR".
package main

/* 6809 Emulator "GOMAR".

   License: GNU General Public License version 2, see LICENSE for more details.

   Converted to GO LANG in 2019 by Henry Strickland (github.com/strickyak)
   based on code with the following copyleft:

   ============================================================================

   created 1994 by L.C. Benschop.
   copyleft (c) 1994-2014 by the sbc09 team, see AUTHORS for more details.
   license: GNU General Public License version 2, see LICENSE for more details.

   This program simulates a 6809 processor.

   System dependencies: short must be 16 bits.
                        char  must be 8 bits.
                        long must be more than 16 bits.
                        arrays up to 65536 bytes must be supported.
                        machine must be twos complement.
   Most Unix machines will work. For MSODS you need long pointers
   and you may have to malloc() the mem array of 65536 bytes.

   Define CPU_BIG_ENDIAN with != 0 if you have a big-endian machine (680x0 etc)
   Usually UNIX systems get this automatically from BIG_ENDIAN and BYTE_ORDER
   definitions ...

   Define TRACE if you want an instruction trace on stderr.
   Define TERM_CONTROL if you want nonblocking non-echoing key input.
   * THIS IS DIRTY !!! *

   Special instructions:
   SWI2 writes char to stdout from register B.
   SWI3 reads char from stdout to register B, sets carry at EOF.
               (or when no key available when using term control).
   SWI retains its normal function.
   CWAI and SYNC stop simulator.

   The program reads a binary image file at $100 and runs it from there.
   The file name must be given on the command line.

   Revisions:
        2012-06-05 johann AT klasek at
                Fixed: com with C "NOT" operator ... 0^(value) did not work!
        2012-06-06
                Fixed: changes from 1994 release (flag handling)
                        reestablished.
        2012-07-15 JK
                New: option parsing, new option -d (dump memory on exit)
        2013-10-07 JK
                New: print ccreg with flag name in lower/upper case depending on flag state.
        2013-10-20 JK
                New: Show instruction disassembling in trace mode.
        2014-07-01 JK
                Fixed: disassembling output: cmpd
        2014-07-11 JK
                Fixed: undocumented tfr/exg register combinations.
                        http://www.6809.org.uk/dragon/illegal-opcodes.shtml

        2016-02-06 Henry Strickland <strickyak>
                Because OS/9 uses SWI2 for kernel calls, allow other SWIs for I/O.
                -i={0,1,2,3} Input char on {none, SWI, SWI2, or SWI3}.
                -o={0,1,2,3} Output char on {none, SWI, SWI2, or SWI3}
                -0  Initialize mem to 00.
                -F  Initialize mem to FF.
                -t  Enable trace.  (Still requires -DTRACE).
                And more.
*/

import (
	"github.com/strickyak/jerbo/gomar/emu"

	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

var FlagCpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
var FlagMemProfile = flag.String("memprofile", "", "write cpu profile to file")

func main() {
	log.SetFlags(0)
	log.SetPrefix("## ")
	flag.Parse()

	if *FlagCpuProfile != "" {
		f, err := os.Create(*FlagCpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	emu.Main()

	if *FlagMemProfile != "" {
		f, err := os.Create(*FlagMemProfile)
		if err != nil {
			log.Fatalf("could not create memory profile: %v", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatalf("could not write memory profile: %v", err)
		}
	}
}
