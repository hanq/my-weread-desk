//go:build windows

package platform

import (
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// logFont is LOGFONTW, 92 bytes.
type logFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
}

var (
	fontOnce sync.Once
	fontList []string
)

// FontFamilies enumerates installed font families once and caches the result.
func FontFamilies() []string {
	fontOnce.Do(func() {
		fontList = enumerateFonts()
	})
	return fontList
}

func enumerateFonts() []string {
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return curatedFonts[:]
	}
	defer procReleaseDC.Call(0, hdc)

	var lf logFont
	lf.CharSet = 1 // DEFAULT_CHARSET

	seen := map[string]bool{}
	var out []string

	cb := windowsCallback(func(elfe uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
		lfp := (*logFont)(unsafe.Pointer(elfe))
		name := utf16ToString(lfp.FaceName[:])
		if name == "" || strings.HasPrefix(name, "@") {
			return 1
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
		return 1
	})
	procEnumFontFamiliesEx.Call(hdc, uintptr(unsafe.Pointer(&lf)), cb, 0, 0)

	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := fontPriority(out[i]), fontPriority(out[j])
		if pi != pj {
			return pi < pj
		}
		return out[i] < out[j]
	})

	// Always offer the curated Chinese families first, even on machines that
	// report them under a localised name.
	var merged []string
	for _, f := range curatedFonts {
		if !seen[f] {
			seen[f] = true
			merged = append(merged, f)
		}
	}
	return append(merged, out...)
}

func fontPriority(name string) int {
	for i, f := range curatedFonts {
		if name == f {
			return i
		}
	}
	return len(curatedFonts)
}

func utf16ToString(buf []uint16) string {
	var b strings.Builder
	for _, v := range buf {
		if v == 0 {
			break
		}
		b.WriteRune(rune(v))
	}
	return b.String()
}

var curatedFonts = []string{
	"Source Han Serif SC",
	"Source Han Sans SC",
	"思源宋体",
	"思源黑体",
	"Microsoft YaHei",
	"微软雅黑",
	"SimSun",
	"宋体",
	"KaiTi",
	"楷体",
	"FangSong",
	"仿宋",
	"SimHei",
	"黑体",
	"NSimSun",
	"等线",
	"DengXian",
	"Songti SC",
	"PingFang SC",
	"Noto Serif CJK SC",
	"Noto Sans CJK SC",
	"LXGW WenKai",
	"霞鹜文楷",
	"Cascadia Mono",
	"Cascadia Code",
	"Consolas",
	"JetBrains Mono",
	"Courier New",
	"Georgia",
	"Times New Roman",
	"Palatino Linotype",
	"Segoe UI",
}
