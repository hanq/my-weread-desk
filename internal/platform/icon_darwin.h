#ifndef ICON_DARWIN_H
#define ICON_DARWIN_H

// cbmSetApplicationIcon loads an image file and installs it as the shared
// NSApplication icon. Returns 1 when the image decoded and the main-thread
// update was scheduled, 0 otherwise. Safe to call from any goroutine.
int cbmSetApplicationIcon(const char *path);

#endif
