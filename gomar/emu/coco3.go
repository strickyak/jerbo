// +build coco3

package emu

import (
	"github.com/strickyak/jerbo/gomar/display"
	"github.com/strickyak/jerbo/gomar/sym"

	"bytes"
	"fmt"
	"log"
	"strings"
)

// While booting OS9 Level2, the screen seems to be doubleByte
// at 07c000 to 07d000.  Second line begins at 07c0a0,
// that is 160 bytes from start, or 80 doubleBytes.
// 4096 div 160 is 25.6 lines.

const P_Path = sym.P_Path

const MmuDefaultStartAddr = (0x38 << 13)

const TraceMem = false // TODO: restore this some day.

var GimeVertIrqEnable bool
var MmuEnable bool
var MmuTask byte
var MmuMap [2][8]byte

func InitHardware() {
	Coco3Contract()
}

var videoEpoch int64

func FireTimerInterrupt() {
	if GimeVertIrqEnable {
		irqs_pending |= IRQ_PENDING
		Waiting = false
	}
	videoEpoch++
	if videoEpoch%10 == 1 {
		PublishVideoText()
	}
}

// Coco3Contract ensures the contract between Coco3's disk booting mechanism
// and the OS/9 Level2 kernel, documented at
// nitros9/level2/modules/kernel/ccbkrn.txt
func Coco3Contract() {

	// Initialize Memory Map thus: 00 39 3a 3b 3c 3d 3e 3f
	for task := 0; task < 2; task++ {
		MmuMap[task][0] = 0x00 // Exception.
		for page := 1; page < 8; page++ {
			MmuMap[task][page] = byte(0x38 + page)
		}
	}
	// Initialize physical block 3b to spaces, except 0x0008 at the beginning.
	const block3b = 0x3b * 0x2000
	mem[block3b+0] = 0x00
	mem[block3b+1] = 0x08
	for i := 2; i < 0x2000; i++ {
		mem[block3b+i] = ' '
	}
	/*   starting at 0xff90:
	6c      init0
	00      init1
	00      irq enable
	00      firq enable
	0900    timer register
	0000    unused
	0320    screen settings
	0000    ????
	00      ????
	ec01    physical video address (block 3b offset 0x0008 )
	00      horizontal offset / scroll

	A mirror of these bytes will appear at 0x0090-0x009f in the DP
	*/
	for i, b := range []byte{0x6c, 0, 0, 0, 9, 0, 0, 0, 3, 0x20, 0, 0, 0, 0x3c, 1, 0} {
		PutIOByte(Word(0xFF90+i), b)
		mem[0x90+i] = b // Probably don't need to set the mirror, but doing it anyway.
	}
}

type Mapping [8]Word

func GetMapping(addr Word) Mapping {
	// Mappings are in SysMem (block 0).
	return Mapping{
		// TODO: drop the "0x3F &".
		0x3F & SysMemW(addr),
		0x3F & SysMemW(addr+2),
		0x3F & SysMemW(addr+4),
		0x3F & SysMemW(addr+6),
		0x3F & SysMemW(addr+8),
		0x3F & SysMemW(addr+10),
		0x3F & SysMemW(addr+12),
		0x3F & SysMemW(addr+14),
	}
}

func WithMmuTask(task byte, fn func()) {
	tmp := MmuTask
	MmuTask = task
	defer func() {
		MmuTask = tmp
	}()
	fn()
}

func GetMappingTask0(addr Word) Mapping {
	// Use Task 0 for the mapping.
	tmp := MmuTask
	MmuTask = 0
	defer func() {
		MmuTask = tmp
	}()

	return Mapping{
		// TODO: drop the "0x3F &".
		0x3F & PeekW(addr),
		0x3F & PeekW(addr+2),
		0x3F & PeekW(addr+4),
		0x3F & PeekW(addr+6),
		0x3F & PeekW(addr+8),
		0x3F & PeekW(addr+10),
		0x3F & PeekW(addr+12),
		0x3F & PeekW(addr+14),
	}
}
func PeekBWithMapping(addr Word, m Mapping) byte {
	logBlock := (addr >> 13) & 7
	physBlock := m[logBlock]
	ptr := int(addr&0x1FFF) | (int(physBlock) << 13)
	return mem[ptr]
}
func PeekWWithMapping(addr Word, m Mapping) Word {
	hi := PeekBWithMapping(addr, m)
	lo := PeekBWithMapping(addr+1, m)
	return (Word(hi) << 8) | Word(lo)
}

func Os9StringWithMapping(addr Word, m Mapping) string {
	var buf bytes.Buffer
	for {
		var b byte = PeekBWithMapping(addr, m)
		var ch byte = 0x7F & b
		if '!' <= ch && ch <= '~' {
			buf.WriteByte(ch)
		} else {
			break
		}
		if (b & 128) != 0 {
			break
		}
		addr++
	}
	return buf.String()
}

func ExplainMMU() string {
	return F("mmu:%d task:%d [[ %02x %02x %02x %02x  %02x %02x %02x %02x || %02x %02x %02x %02x  %02x %02x %02x %02x ]]",
		CondI(MmuEnable, 1, 0),
		MmuTask&1,
		MmuMap[0][0],
		MmuMap[0][1],
		MmuMap[0][2],
		MmuMap[0][3],
		MmuMap[0][4],
		MmuMap[0][5],
		MmuMap[0][6],
		MmuMap[0][7],
		MmuMap[1][0],
		MmuMap[1][1],
		MmuMap[1][2],
		MmuMap[1][3],
		MmuMap[1][4],
		MmuMap[1][5],
		MmuMap[1][6],
		MmuMap[1][7],
	)
}

func MapAddrWithMapping(logical Word, m Mapping) int {
	slot := 7 & (logical >> 13)
	low := int(logical & 0x1FFF)
	physicalPage := m[slot]
	return (int(physicalPage) << 13) | low
}

func MapAddr(logical Word, quiet bool) int {
	if logical >= 0xFE00 {
		return (0x3F << 13) | int(logical)
	}
	var z int
	if MmuEnable {
		slot := byte(logical >> 13)
		low := int(logical & 0x1FFF)
		physicalPage := MmuMap[MmuTask][slot]
		z = (int(physicalPage) << 13) | low
		if !quiet && TraceMem {
			L("\t\t\t\t\t\t MapAddr: %04x -> %06x ... task=%x  slot=%x  page=%x", logical, z, MmuTask, slot, physicalPage)
		}
		return z
	} else {
		if z < 0x2000 {
			z = int(logical)
		} else {
			z = MmuDefaultStartAddr + int(logical)
		}
		if !quiet && TraceMem {
			L("\t\t\t\t\t\t MapAddr: %04x -> %06x ... default map", logical, z)
		}
		return z
	}
}

// B is fundamental func to get byte.  Hack register access into here.
func B(addr Word) byte {
	var z byte
	mapped := MapAddr(addr, false)
	if AddressInDeviceSpace(addr) {
		z = GetIOByte(addr)
		LogIO("GetIO (%06x) %04x -> %02x : %c %c", mapped, addr, z, H(z), T(z))
		mem[mapped] = z
	} else {
		z = mem[mapped]
	}
	if TraceMem {
		L("\t\t\t\tGetB (%06x) %04x -> %02x : %c %c", mapped, addr, z, H(z), T(z))
	}
	return z
}

func PokeB(addr Word, b byte) {
	mapped := MapAddr(addr, true)
	mem[mapped] = b
}

func PeekB(addr Word) byte {
	var z byte
	mapped := MapAddr(addr, true)
	z = mem[mapped]
	return z
}

// PutB is fundamental func to set byte.  Hack register access into here.
func PutB(addr Word, x byte) {
	mapped := MapAddr(addr, false)
	old := mem[mapped]
	mem[mapped] = x
	if TraceMem {
		LogIO("\t\t\t\tPutB (%06x) %04x <- %02x (was %02x)", mapped, addr, x, old)
	}
	if AddressInDeviceSpace(addr) {
		PutIOByte(addr, x)
		LogIO("PutIO (%06x) %04x <- %02x (was %02x)", mapped, addr, x, old)
	}
}

func PeekWPhys(addr int) Word {
	if addr+1 > len(mem) {
		panic(addr)
		// return 0
	}
	return Word(mem[addr])<<8 | Word(mem[addr+1])
}

//////// DUMP

func DoDumpAllMemoryPhys() {
	if !V['p'] {
		return
	}
	var i, j int
	var buf bytes.Buffer
	L("\n#DumpAllMemoryPhys(\n")
	n := len(mem)
	for i = 0; i < n; i += 32 {
		if i&0x1FFF == 0 {
			L("P [%02x] %06x:", i>>13, i)
		}
		// Look ahead for something interesting on this line.
		something := false
		for j = 0; j < 32; j++ {
			x := mem[i+j]
			// if x != 0 && x != ' ' //
			if x != 0 {
				something = true
				break
			}
		}

		if !something {
			continue
		}

		buf.Reset()
		Z(&buf, "P %06x: ", i)
		for j = 0; j < 32; j += 8 {
			Z(&buf,
				"%02x%02x %02x%02x %02x%02x %02x%02x  ",
				mem[i+j+0], mem[i+j+1], mem[i+j+2], mem[i+j+3],
				mem[i+j+4], mem[i+j+5], mem[i+j+6], mem[i+j+7])
		}
		buf.WriteRune(' ')
		for j = 0; j < 32; j++ {
			ch := 0x7F & mem[i+j]
			var r rune = '.'
			if ' ' <= ch && ch <= '~' {
				r = rune(ch)
			}
			buf.WriteRune(r)
		}
		L("%s\n", buf.String())
	}
	L("#DumpAllMemoryPhys)\n")
}

func DoExplainMmuBlock(i int) {
	blk := (i >> 13) & 0x3F
	blkPhys := MmuMap[MmuTask][blk]
	L("[%x -> %02x] %06x", blk, blkPhys, MapAddr(Word(i), true))
}

func DoDumpPageZero() {
	mmut := MmuTask
	MmuTask = 0
	map00 := MmuMap[0][0]
	MmuMap[0][0] = 0
	defer func() {
		MmuTask = mmut
		MmuMap[0][0] = map00
	}()

	L("PageZero:\n")

	/* some Level1:
	L("PageZero: FreeBitMap=%x:%x MemoryLimit=%x ModDir=%x RomBase=%x\n",
		W(sym.D_FMBM), W(sym.D_FMBM+2), W(sym.D_MLIM), W(sym.D_ModDir), W(sym.D_Init))
	*/
	L("  D_SWI3=%x D_SWI2=%x FIRQ=%x IRQ=%x SWI=%x NMI=%x SvcIRQ=%x Poll=%x\n",
		W(sym.D_SWI3), W(sym.D_SWI2), W(sym.D_FIRQ), W(sym.D_IRQ), W(sym.D_SWI), W(sym.D_NMI), W(sym.D_SvcIRQ), W(sym.D_Poll))
	/* some Level1:
	L("  BTLO=%x BTHI=%x  IO Free Mem Lo=%x Hi=%x D_DevTbl=%x D_PolTbl=%x D_PthDBT=%x D_Proc=%x\n",
		W(sym.D_BTLO), W(sym.D_BTHI), W(sym.D_IOML), W(sym.D_IOMH), W(sym.D_DevTbl), W(sym.D_PolTbl), W(sym.D_PthDBT), W(sym.D_Proc))
	*/
	L("  D_Slice=%x D_TSlice=%x\n",
		W(sym.D_Slice), W(sym.D_TSlice))

	var buf bytes.Buffer
	Z(&buf, " D.Tasks=%04x", PeekW(sym.D_Tasks))
	Z(&buf, " D.TmpDAT=%04x", PeekW(sym.D_TmpDAT))
	Z(&buf, " D.Init=%04x", PeekW(sym.D_Init))
	Z(&buf, " D.Poll=%04x", PeekW(sym.D_Poll))
	Z(&buf, " D.Tick=%02x", PeekB(sym.D_Tick))
	Z(&buf, " D.Slice=%02x", PeekB(sym.D_Slice))
	Z(&buf, " D.TSlice=%02x", PeekB(sym.D_TSlice))
	Z(&buf, " D.Boot=%02x", PeekB(sym.D_Boot))
	Z(&buf, " D.MotOn=%02x", PeekB(sym.D_MotOn))
	Z(&buf, " D.ErrCod=%02x", PeekB(sym.D_ErrCod))
	Z(&buf, " D.Daywk=%02x", PeekB(sym.D_Daywk))
	Z(&buf, " D.TkCnt=%02x", PeekB(sym.D_TkCnt))
	Z(&buf, " D.BtPtr=%04x", PeekW(sym.D_BtPtr))
	Z(&buf, " D.BtSz=%04x", PeekW(sym.D_BtSz))
	L("%s", buf.String())
	buf.Reset()

	Z(&buf, " D.CRC=%02x", PeekB(sym.D_CRC))
	Z(&buf, " D.Tenths=%02x", PeekB(sym.D_Tenths))
	Z(&buf, " D.Task1N=%02x", PeekB(sym.D_Task1N))
	Z(&buf, " D.Quick=%02x", PeekB(sym.D_Quick))
	Z(&buf, " D.QIRQ=%02x", PeekB(sym.D_QIRQ))
	Z(&buf, " D.BlkMap=%04x,%04x", PeekW(sym.D_BlkMap), PeekW(sym.D_BlkMap+2))
	Z(&buf, " D.ModDir=%04x,%04x", PeekW(sym.D_ModDir), PeekW(sym.D_ModDir+2))
	Z(&buf, " D.PrcDBT=%04x", PeekW(sym.D_PrcDBT))
	Z(&buf, " D.SysPrc=%04x", PeekW(sym.D_SysPrc))
	Z(&buf, " D.SysDAT=%04x", PeekW(sym.D_SysDAT))
	// Z(&buf, " D.Mem=%04x", PeekW(sym.D_Mem))
	Z(&buf, " D.Proc=%04x", PeekW(sym.D_Proc))
	Z(&buf, " D.AProcQ=%04x", PeekW(sym.D_AProcQ))
	Z(&buf, " D.WProcQ=%04x", PeekW(sym.D_WProcQ))
	Z(&buf, " D.SProcQ=%04x", PeekW(sym.D_SProcQ))
	L("%s", buf.String())
	buf.Reset()

	Z(&buf, " D.ModEnd=%04x", PeekW(sym.D_ModEnd))
	Z(&buf, " D.ModDAT=%04x", PeekW(sym.D_ModDAT))
	Z(&buf, " D.CldRes=%04x", PeekW(sym.D_CldRes))
	Z(&buf, " D.BtBug=%04x%02x", PeekW(sym.D_BtBug), PeekB(sym.D_BtBug+2))
	Z(&buf, " D.Pipe=%04x", PeekW(sym.D_Pipe))

	Z(&buf, " D.QCnt=%02x", PeekB(sym.D_QCnt))
	Z(&buf, " D.DevTbl=%04x", PeekW(sym.D_DevTbl))
	Z(&buf, " D.PolTbl=%04x", PeekW(sym.D_PolTbl))
	Z(&buf, " D.PthDBT=%04x", PeekW(sym.D_PthDBT))
	Z(&buf, " D.DMAReq=%02x", PeekB(sym.D_DMAReq))
	L("%s", buf.String())
	buf.Reset()
}

func DoDumpPathDesc(a Word) {
	L("a=%04x", a)
	if 0 == B(a+sym.PD_PD) {
		return
	}
	L("Path @%x: #=%x mode=%x count=%x dev=%x\n", a, B(a+sym.PD_PD), B(a+sym.PD_MOD), B(a+sym.PD_CNT), W(a+sym.PD_DEV))
	L("   curr_process=%x caller_reg_stack=%x buffer=%x  dev_type=%x\n",
		B(a+sym.PD_CPR), B(a+sym.PD_RGS), B(a+sym.PD_BUF), B(a+sym.PD_DTP))

	// the Device Table Entry:
	dev := W(a + sym.PD_DEV)
	var buf bytes.Buffer
	Z(&buf, "   dev: @%x driver_mod=%x=%s ",
		dev, W(dev+sym.V_DRIV), ModuleName(W(dev+sym.V_DRIV)))
	Z(&buf, "driver_static_store=%x descriptor_mod=%x=%s ",
		W(dev+sym.V_STAT), W(dev+sym.V_DESC), ModuleName(W(dev+sym.V_DESC)))
	Z(&buf, "file_man=%x=%s use=%d\n",
		W(dev+sym.V_FMGR), ModuleName(W(dev+sym.V_FMGR)), B(dev+sym.V_USRS))
	L("%s", buf.String())

	if false && paranoid {
		if B(a+sym.PD_PD) > 10 {
			panic("PD_PD")
		}
		if B(a+sym.PD_CNT) > 20 {
			panic("PD_CNT")
		}
		if B(a+sym.PD_CPR) > 10 {
			panic("PD_CPR")
		}
	}
}

func DoDumpAllPathDescs() {
	if true || Level == 1 {
		p := W(sym.D_PthDBT)
		if 0 == p {
			return
		}

		for i := Word(0); i < 32; i++ {
			q := W(p + i*2)
			if q != 0 {
				L("PathDesc[%x]: %x", i, q)

				for j := Word(0); j < 4; j++ {
					k := i*4 + j
					L("........[%x]: %x", j, k)
					if k == 0 {
						continue
					} // There is no path desc 0 (it's the table).
					DoDumpPathDesc(q + j*64)
				}

			}
		}
	}
}

func DoDumpProcDesc(a Word) {
	tmp := MmuTask
	MmuTask = 0
	defer func() {
		MmuTask = tmp
	}()

	L("a=%04x", a)
	switch Level {
	case 1, 2:
		{
			begin := PeekW(a + sym.P_PModul)
			name_str := "?"
			mod_str := "?"
			if begin != 0 {
				if Level == 1 {
					name := begin + PeekW(begin+4)
					name_str = Os9String(name)
					mod_str = F("%q @%04x", name_str, begin)
				} else if Level == 2 {
					m := GetMappingTask0(a + sym.P_DATImg)
					modPhys := MapAddrWithMapping(begin, m)
					modPhysPlus4 := PeekWPhys(modPhys + 4)
					if modPhysPlus4 > 0 {
						name := begin + modPhysPlus4
						name_str = Os9StringWithMapping(name, m)
						mod_str = F("%q @%04x %v", name_str, begin, m)
					}
				}
			}
			L("Process @%x: id=%x pid=%x sid=%x cid=%x module=%s", a, B(a+sym.P_ID), B(a+sym.P_PID), B(a+sym.P_SID), B(a+sym.P_CID), mod_str)
			/* some Level1
			L("   sp=%x chap=%x Addr=%x PagCnt=%x User=%x Pri=%x Age=%x State=%x",
				W(a+sym.P_SP), B(a+sym.P_CHAP), B(a+sym.P_ADDR), B(a+sym.P_PagCnt), W(a+sym.P_User), B(a+sym.P_Prior), B(a+sym.P_Age), B(a+sym.P_State))
			*/
			L("   Queue=%x IOQP=%x IOQN=%x Signal=%x SigVec=%x SigDat=%x",
				W(a+sym.P_Queue), B(a+sym.P_IOQP), B(a+sym.P_IOQN), B(a+sym.P_Signal), B(a+sym.P_SigVec), B(a+sym.P_SigDat))
			L("   DIO %x %x %x %x %x %x PATH %x %x %x %x %x %x %x %x %x %x %x %x %x %x %x %x",
				W(a+sym.P_DIO), W(a+sym.P_DIO+2), W(a+sym.P_DIO+4),
				W(a+sym.P_DIO+6), W(a+sym.P_DIO+8), W(a+sym.P_DIO+10),
				B(a+sym.P_Path+0), B(a+sym.P_Path+1), B(a+sym.P_Path+2), B(a+sym.P_Path+3),
				B(a+sym.P_Path+4), B(a+sym.P_Path+5), B(a+sym.P_Path+6), B(a+sym.P_Path+7),
				B(a+sym.P_Path+8), B(a+sym.P_Path+9), B(a+sym.P_Path+10), B(a+sym.P_Path+11),
				B(a+sym.P_Path+12), B(a+sym.P_Path+13), B(a+sym.P_Path+14), B(a+sym.P_Path+15))
			if W(a+sym.P_Queue) != 0 {
				// If current proc, it has no queue.
				// Other procs are in a queue.
				if W(sym.D_Proc) != a {
					log.Printf("BUG printing PROC: %x", a)
					return
					DoDumpProcDesc(W(a + sym.P_Queue))
				}
			}

			if paranoid {
				if B(a+sym.P_ID) > 10 {
					panic("P_ID")
				}
				if B(a+sym.P_PID) > 10 {
					panic("P_PID")
				}
				if B(a+sym.P_SID) > 10 {
					panic("P_SID")
				}
				if B(a+sym.P_CID) > 10 {
					panic("P_CID")
				}
				if W(a+sym.P_User) > 10 {
					panic("P_User")
				}
			}
		}
	}
}

func DoDumpProcesses() {
	if W(sym.D_Proc) != 0 {
		L("D_Proc: CURRENT:")
		DoDumpProcDesc(W(sym.D_Proc))
	}
	if W(sym.D_AProcQ) != 0 {
		L("D_AProcQ: Active:")
		DoDumpProcDesc(W(sym.D_AProcQ))
	}
	if W(sym.D_WProcQ) != 0 {
		L("D_WProcQ: Wait:")
		DoDumpProcDesc(W(sym.D_WProcQ))
	}
	if W(sym.D_SProcQ) != 0 {
		L("D_SProcQ: Sleep")
		DoDumpProcDesc(W(sym.D_SProcQ))
	}
}

func LPeekB(a Word) uint64 {
	return uint64(PeekB(a))
}

func ExplainColor(b byte) string {
	return F("rgb=$%02x=(%x,%x,%x)", b&63,
		((b&0x20)>>4)|((b&0x04)>>2),
		((b&0x10)>>3)|((b&0x02)>>1),
		((b&0x08)>>2)|((b&0x01)>>0))
}

/*
HRES:
	http://users.axess.com/twilight/sock/gime.html
Horizontal resolution using graphics:
000=16 bytes per row
001=20 bytes per row
010=32 bytes per row
011=40 bytes per row
100=64 bytes per row
101=80 bytes per row
110=128 bytes per row
111=160 bytes per row

When using text:
0x0=32 characters per row
0x1=40 characters per row
1x0=64 characters per row
1x1=80 characters per row
*/

var GraphicsBytesPerRowHRES = []int{16, 20, 32, 40, 64, 80, 128, 160}
var AlphaCharsPerRowHRES = []int{32, 40, 32, 40, 64, 80, 64, 80}

/*
CRES:
	http://users.axess.com/twilight/sock/gime.html
Color Resolution using graphics:
00=2 colors (8 pixels per byte)
01=4 colors (4 pixels per byte)
10=16 colors (2 pixels per byte)
11=Undefined (would have been 256 colors)

When using text:
x0=No color attributes
x1=Color attributes enabled
*/

var GraphicsColorBitsCRES = []int{1, 2, 4, 8}
var AlphaHasAttrsCRES = []bool{false, true, false, true}

var FF92Bits = []string{
	"?", "?", "TimerIRQ", "HorzIRQ", "VertIRQ", "SerialIRQ", "KbdIRQ", "CartIRQ"}
var FF93Bits = []string{
	"?", "?", "TimerFIRQ", "HorzFIRQ", "VertFIRQ", "SerialFIRQ", "KbdFIRQ", "CartFIRQ"}

var GimeLinesPerField = []int{192, 200, 210, 225}
var GimeLinesPerCharRow = []int{1, 2, 3, 8, 9, 10, 12, -1}

func GetCocoDisplayParams() *display.CocoDisplayParams {
	a := PeekB(0xFF98)
	b := PeekB(0xFF99)
	c := PeekB(0xFF9C)
	d := PeekB(0xFF9F)
	z := &display.CocoDisplayParams{
		Gime:            true,
		Graphics:        (a>>7)&1 != 0,
		AttrsIfAlpha:    (a>>6)&1 != 0,
		VirtOffsetAddr:  int(HiLo(PeekB(0xFF9D), PeekB(0xFF9E))) << 3,
		HorzOffsetAddr:  int(d & 127),
		VirtScroll:      int(c & 15),
		LinesPerField:   GimeLinesPerField[(b>>5)&3],
		LinesPerCharRow: GimeLinesPerCharRow[a&7],
		Monochrome:      (a>>4)&1 != 0,
		HRES:            int((b >> 2) & 7),
		CRES:            int(b & 3),
		HVEN:            d>>7 != 0,
	}
	if z.Graphics {
		z.GraphicsBytesPerRow = GraphicsBytesPerRowHRES[z.HRES]
		z.GraphicsColorBits = GraphicsColorBitsCRES[z.CRES]
	} else {
		z.AlphaCharsPerRow = AlphaCharsPerRowHRES[z.HRES]
		z.AlphaHasAttrs = AlphaHasAttrsCRES[z.CRES]
	}
	for i := 0; i < 16; i++ {
		z.ColorMap[i] = PeekB(0xFFB0 + Word(i))
	}
	return z
}

func DumpGimeStatus() {
	for i := Word(0); i < 16; i += 4 {
		L("GIME/palette[%x..%x]: %s %s %s %s", i, i+3,
			ExplainColor(PeekB(0xFFB0+i)),
			ExplainColor(PeekB(0xFFB1+i)),
			ExplainColor(PeekB(0xFFB2+i)),
			ExplainColor(PeekB(0xFFB3+i)))
	}
	L("GIME/CpuSpeed: %x", PeekB(0xFFD9))
	L("GIME/MmuEnable: %v", PeekB(0xFF90)&0x40 != 0)
	L("GIME/MmuTask: %v; clock rate: %v", MmuTask, 0 != (PeekB(0xFF91)&0x40))
	L("GIME/IRQ bits: %s", ExplainBits(PeekB(0xFF92), FF92Bits))
	L("GIME/FIRQ bits: %s", ExplainBits(PeekB(0xFF93), FF93Bits))
	L("GIME/Timer=$%x", HiLo(PeekB(0xFF94), PeekB(0xFF95)))
	b := PeekB(0xFF98)
	L("GIME/GraphicsNotAlpha=%x AttrsIfAlpha=%x Artifacting=%x Monochrome=%x 50Hz=%x LinesPerCharRow=%x=%d.",
		(b>>7)&1,
		(b>>6)&1,
		(b>>5)&1,
		(b>>4)&1,
		(b>>3)&1,
		(b & 7),
		GimeLinesPerCharRow[b&7])
	b = PeekB(0xFF99)
	L("GIME/LinesPerField=%x=%d. HRES=%x CRES=%x",
		(b>>5)&3,
		GimeLinesPerField[(b>>5)&3],
		(b>>2)&7,
		b&3)

	b = PeekB(0xFF9C)
	L("GIME/Virt Scroll (alpha) = %x", b&15)
	L("GIME/VirtOffsetAddr=$%05x",
		uint64(HiLo(PeekB(0xFF9D), PeekB(0xFF9E)))<<3)
	/*
		L("GIME/VirtOffsetAddr=$%05x",
				(((LPeekB(0xFF9C)>>4)&7)<<16)|
					(((LPeekB(0xFF9D))&255)<<8)|
					(((LPeekB(0xFF9E))&255)<<0))
	*/
	b = PeekB(0xFF9F)
	L("GIME/HVEN=%x HorzOffsetAddr=%x", (b >> 7), b&127)
	L("GIME/GetCocoDisplayParams = %#v", *GetCocoDisplayParams())
}

func PutGimeIOByte(a Word, b byte) {
	PokeB(a, b)

	switch a {
	default:
		log.Panicf("UNKNOWN PutIOByte address: 0x%04x", a)

	case 0xFFB0,
		0xFFB1,
		0xFFB2,
		0xFFB3,
		0xFFB4,
		0xFFB5,
		0xFFB6,
		0xFFB7,
		0xFFB8,
		0xFFB9,
		0xFFBA,
		0xFFBB,
		0xFFBC,
		0xFFBD,
		0xFFBE,
		0xFFBF:
		L("GIME\t\t$%x: palette[$%x] <- %s", a, a&15, ExplainColor(b))

	case 0xFFD9:
		L("GIME\t\t$%x: Cpu Speed <- %02x", a, b)

	case 0xFF90:
		MmuEnable = 0 != (b & 0x40)
		L("GIME MmuEnable <- %v", MmuEnable)

	case 0xFF91:
		MmuTask = b & 0x01
		L("GIME MmuTask <- %v; clock rate <- %v", MmuTask, 0 != (b&0x40))

	case 0xFF92:
		L("GIME\t\tIRQ bits: %s", ExplainBits(b, FF92Bits))
		// 0x08: Vertical IRQ.  0x01: Cartridge.
		if (b &^ 0x09) != 0 {
			log.Panicf("GIME IRQ Enable for unsupported emulated bits: %04x %02x", a, b)
		}
		if (b & 0x08) != 0 {
			GimeVertIrqEnable = true
		} else {
			GimeVertIrqEnable = false
		}

	case 0xFF93:
		L("GIME\t\tFIRQ bits: %s", ExplainBits(b, FF93Bits))
		if b != 0 {
			log.Panicf("GIME FIRQ Enable for unsupported emulated bits: %04x %02x", a, b)
		}

	case 0xFF94:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tTimer=$%x Start!", HiLo(PeekB(0xFF94), PeekB(0xFF95)))
	case 0xFF95:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tTimer=$%x", HiLo(PeekB(0xFF94), PeekB(0xFF95)))
	case 0xFF96:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\treserved")
	case 0xFF97:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\treserved")
	case 0xFF98:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tGraphicsNotAlpha=%x AttrsIfAlpha=%x Artifacting=%x Monochrome=%x 50Hz=%x LinesPerCharRow=%x=%d.",
			(b>>7)&1,
			(b>>6)&1,
			(b>>5)&1,
			(b>>4)&1,
			(b>>3)&1,
			(b & 7),
			GimeLinesPerCharRow[b&7])
	case 0xFF99:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tLinesPerField=%x=%d. HRES=%x CRES=%x",
			(b>>5)&3,
			GimeLinesPerField[(b>>5)&3],
			(b>>2)&7,
			b&3)

	case 0xFF9A:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tBorder: ", ExplainColor(b))
	case 0xFF9B:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tNot Used")
	case 0xFF9C:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tVirt Scroll (alpha) = %x", b&15)
	case 0xFF9D,
		0xFF9E:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tVirtOffsetAddr=$%05x",
			uint64(HiLo(PeekB(0xFF9D), PeekB(0xFF9E)))<<3)
	case 0xFF9F:
		L("GIME %x <= %02x", a, b)
		L("GIME\t\tHVEN=%x HorzOffsetAddr=%x", (b >> 7), b&127)

	case 0xFFA0,
		0xFFA1,
		0xFFA2,
		0xFFA3,
		0xFFA4,
		0xFFA5,
		0xFFA6,
		0xFFA7,
		0xFFA8,
		0xFFA9,
		0xFFAA,
		0xFFAB,
		0xFFAC,
		0xFFAD,
		0xFFAE,
		0xFFAF:
		{
			task := byte((a >> 3) & 1)
			slot := byte(a & 7)
			was := MmuMap[task][slot]
			MmuMap[task][slot] = b & 0x3F
			L("GIME MmuMap[%d][%d] <- %02x  (was %02x)", task, slot, b, was)
			// if task == 0 && slot == 7 && b != 0x3F {
			// panic("bad MmuMap[0][7]")
			// }
			// yak ddt TODO
			// MmuMap[0][7] = 0x3F // Never change slot 7.
			// MmuMap[1][7] = 0x3F // Never change slot 7.
		}

	}
}
func MemoryModuleOf(addr Word) (name string, offset Word) {
	// TODO: speed up with caching.
	if addr >= 0xFF00 {
		log.Panicf("PC in IO page: $%x", addr)
	}
	if addr >= 0xFE00 {
		return "(FE)", 0 // No module found for the addr.
	}
	if addr < 0x0100 {
		return "(00)", 0 // No module found for the addr.
	}

	addrPhys := MapAddr(addr, true)
	addr32 := uint32(addrPhys)

	// First scan for initial modules.
	for _, m := range InitialModules {
		if addr32 >= m.Addr && addr32 < (m.Addr+m.Len) {
			return m.Id(), Word(addr32 - m.Addr)
		}
	}

	dirStart := SysMemW(sym.D_ModDir)
	dirLimit := SysMemW(sym.D_ModEnd)
	for i := dirStart; i < dirLimit; i += 8 {
		datPtr := SysMemW(i + 0)
		// usedBytes := SysMemW(i + 2)
		begin := SysMemW(i + 4)
		links := SysMemW(i + 6)
		if datPtr == 0 {
			continue
		}

		m := GetMapping(datPtr)
		magic := PeekWWithMapping(begin, m)
		if magic != 0x87CD {
			panic(i)
		}
		//log.Printf("DDT: TRY i=%x begin=%x %q .....", i, begin, ModuleId(begin, m))

		// Module offset 2 is module size.
		remaining := int(PeekWWithMapping(begin+2, m))
		// Module offset 4 is offset to name string.
		// namePtr := begin + PeekWWithMapping(begin+4, m)

		//-------------
		// beginP := MapAddrWithMapping(begin, m)

		region := begin
		offset := Word(0) // offset into module.
		for remaining > 0 {
			// If module crosses paged blocks, it has more than one region.
			regionP := MapAddrWithMapping(region, m)
			endOfRegionBlockP := 1 + (regionP | 0x1FFF)
			regionSize := remaining
			if int(regionSize) > endOfRegionBlockP-regionP {
				// A smaller region of the module.
				regionSize = endOfRegionBlockP - regionP
			}

			//log.Printf("DDT: try %x (%x) %x", regionP, addrPhys, regionP+int(regionSize))
			if regionP <= addrPhys && addrPhys < regionP+int(regionSize) {
				if links == 0 {
					log.Panicf("in unlinked module: i=%x addr=%x", i, addr)
				}
				id := ModuleId(begin, m)
				delta := offset + Word(int(addrPhys)-regionP)
				//log.Printf("DDT: FOUND %q+%x", id, delta)
				return id, delta
			}
			remaining -= regionSize
			regionP += regionSize
			region += Word(regionSize)
			offset += Word(regionSize)
			//log.Printf("DDT: advanced remaining=%x regionSize=%x", remaining, regionSize)
		}
	}
	//log.Printf("DDT: NOT FOUND")
	return "", 0 // No module found for the addr.
}

func ModuleId(begin Word, m Mapping) string {
	namePtr := begin + PeekWWithMapping(begin+4, m)
	modname := strings.ToLower(Os9StringWithMapping(namePtr, m))
	sz := PeekWWithMapping(begin+2, m)
	crc1 := PeekBWithMapping(begin+sz-3, m)
	crc2 := PeekBWithMapping(begin+sz-2, m)
	crc3 := PeekBWithMapping(begin+sz-1, m)
	return fmt.Sprintf("%s.%04x%02x%02x%02x", modname, sz, crc1, crc2, crc3)
}

func MemoryModules() {
	if false {
		DumpAllMemory()
		DumpPageZero()
		DumpProcesses()
		DumpAllPathDescs()
		L("\n#MemoryModules(")

		var buf bytes.Buffer
		Z(&buf, "MOD name begin:end(len/blocklen) [addr:dat,blocklen,begin,links] dat\n")

		dirStart := SysMemW(sym.D_ModDir)
		dirLimit := SysMemW(sym.D_ModEnd)
		for i := dirStart; i < dirLimit; i += 8 {
			datPtr := SysMemW(i + 0)
			usedBytes := SysMemW(i + 2)
			begin := SysMemW(i + 4)
			links := SysMemW(i + 6)
			if datPtr == 0 {
				continue
			}

			m := GetMapping(datPtr)
			end := begin + PeekWWithMapping(begin+2, m)
			name := begin + PeekWWithMapping(begin+4, m)

			Z(&buf, "MOD %s %x:%x(%x/%x) [%x:%x,%x,%x,%x] %v\n", Os9StringWithMapping(name, m), begin, end, end-begin, usedBytes, i, datPtr, usedBytes, begin, links, m)
		}
		L("%s", buf.String())
		L("#MemoryModules)")
	}
}

func HandleBtBug() {
	if pcreg == sym.D_BtBug {
		if len(DebugString) < 20 {
			DebugString += string(rune(GetAReg() & 0x7F))
		}
	}
}
