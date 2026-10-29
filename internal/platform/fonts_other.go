//go:build !windows

package platform

// FontFamilies returns a curated list on platforms without GDI enumeration.
func FontFamilies() []string {
	return []string{
		"Songti SC", "PingFang SC", "Heiti SC", "Kaiti SC",
		"Noto Serif CJK SC", "Noto Sans CJK SC",
		"Georgia", "Times New Roman", "Menlo", "SF Mono", "monospace",
	}
}
