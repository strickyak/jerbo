/*
  Grok and extract the contnts of a Microware OS-9 (or NitrOS9) disk image (at least, for Motorola 6809).

  OLD Usage:

    go run grok_os9_disk.go [target_dirname] < nitros9/nos96809l1v030208coco_40d_1.dsk

  The command will list and explain the contents of the OS9 disk image on its stdin.

  If an argument is provided, the disk will be unbundled into that Posix directory on your host system.
*/
package undisk

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Segment struct {
	At   int
	Segs int
}
type PairIS struct {
	A int
	B string
}

func ReadAtLen(pos int, sz int) []byte {
	_, err := os.Stdin.Seek(int64(pos), 0)
	if err != nil {
		panic(err)
	}
	return ReadN(os.Stdin, sz)
}
func PrintInode(inode int, path string) {
	if unixFlag {
		path = strings.ToLower(path)
	}
	bb := ReadAtLen(inode*256, 256)

	fmt.Printf("\n========= INODE #%d %q =========\n\n", inode, path)
	PrintRecords(FileDescSector, bb)
	fmt.Println("\n")
	attrs := FindInt(FileDescSector, bb, "FD.ATT")
	size := FindInt(FileDescSector, bb, "FD.SIZ")

	var segments []Segment
	for i := 0; 16+5*i < 256-5+1; i++ {
		bb = ReadAtLen(inode*256+16+5*i, 256)
		at := FindInt(SegmentEntry, bb, "FDSL.A")
		segs := FindInt(SegmentEntry, bb, "FDSL.B")
		if segs == 0 {
			continue
		}

		// Grr. Some flawed disks let me this heuristic hack.
		if at > 1000000 || segs > 1000 {
			fmt.Printf("ERROR: FLAWED SEGMENT TABLE: %d %d\n", at, segs)
			break
		}

		segments = append(segments, Segment{at, segs})
		fmt.Printf("=== Segment %d ===\n", i)
		PrintRecords(SegmentEntry, bb)
	}
	fmt.Println("\n")

	if attrs&0x80 != 0 { // If is a DIR

		var subs []PairIS
		for _, p := range segments {
			fmt.Printf("=== Directory Segment at %d segs %d (inode %d path %q) ===\n", p.At, p.Segs, inode, path)

			if destFlag != "" {
				os.MkdirAll(filepath.Join(destFlag, path), 0755)
			}

			segsize := p.Segs * 256
			if size < segsize {
				segsize, size = size, 0
			} else {
				size -= segsize
			}

			nth := 0
			for i := 0; i*32 < segsize; i++ {
				bb = ReadAtLen(p.At*256+i*32, 32)
				fd := FindInt(DirEntry, bb, "DIR.FD")
				if fd == 0 {
					continue
				}
				PrintRecords(DirEntry, bb)
				name := FindString(DirEntry, bb, "DIR.NM")
				// Grr. Some flawed disks led me to the "nth >= 2" heuristic hack.
				if name != "" && name != "." && name != ".." && nth >= 2 {
					subName := path + "/" + name
					if path == "/" {
						subName = "/" + name
					}
					subs = append(subs, PairIS{fd, subName})
				}
				nth++
			}
		}
		for _, p := range subs {
			PrintInode(p.A, p.B)
		}

	} else { // IF is a FILE
		var contents []byte
		for _, p := range segments {
			fmt.Printf("=== File Segment at %d segs %d (inode %d path %q) ===\n", p.At, p.Segs, inode, path)

			segsize := p.Segs * 256
			if size < segsize {
				segsize, size = size, 0
			} else {
				size -= segsize
			}

			bb := ReadAtLen(p.At*256, segsize)
			contents = append(contents, bb...)
		}

		PrintModuleHeader(contents)

		if destFlag != "" {
			if unixFlag {
				contents = bytes.ReplaceAll(contents, []byte{13}, []byte{10})
			}
			ioutil.WriteFile(filepath.Join(destFlag, path), contents, 0666)
		}
	}
}

func undisk() {
	bb := ReadN(os.Stdin, 256)

	fmt.Println("=== SECTOR ZERO ===\n")
	PrintRecords(Sector0, bb)
	fmt.Println("\n")

	rootInode := FindInt(Sector0, bb, "DD.DIR")
	PrintInode(rootInode, "/")
}

var destFlag string
var unixFlag bool

func Main() {
	flags := flag.NewFlagSet("undisk", 0)
	flags.BoolVar(&unixFlag, "unix", false, "Unixify: use lowercase and change \\r to \\n")
	flags.StringVar(&destFlag, "dest", "", "if non-empty, destination directory where files are written.  Will create dirs if needed.")
	flags.Parse(os.Args[1:])

	undisk()
}
