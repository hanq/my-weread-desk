//go:build darwin

package platform

/*
#cgo darwin LDFLAGS: -framework AppKit -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "icon_darwin.h"
*/
import "C"

// UniformTypeIdentifiers is linked here because the Wails v2.7.1 darwin frontend
// calls [UTType typeWithFilenameExtension:] but only declares Foundation, Cocoa
// and WebKit in its own cgo LDFLAGS, so the symbol would otherwise be undefined.

import (
	"fmt"
	"unsafe"
)

// setWindowIcon replaces the application icon shown in the Dock and the
// application switcher. AppKit must build the NSImage and touch the shared
// NSApplication on the main thread, so the Objective-C helper hops there; the
// file path is only valid for the duration of the call.
func setWindowIcon(path string) error {
	if path == "" {
		return nil
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	if C.cbmSetApplicationIcon(cpath) == 0 {
		return fmt.Errorf("platform: NSImage 无法读取图标 %s", path)
	}
	return nil
}
