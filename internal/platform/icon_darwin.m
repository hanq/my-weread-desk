#import <AppKit/AppKit.h>

#import "icon_darwin.h"

int cbmSetApplicationIcon(const char *path) {
	if (path == NULL) {
		return 0;
	}
	@autoreleasepool {
		NSString *file = [NSString stringWithUTF8String:path];
		if (file == nil) {
			return 0;
		}
		NSImage *image = [[NSImage alloc] initWithContentsOfFile:file];
		if (image == nil) {
			return 0;
		}
		// NSApplication is UI state and must be mutated on the main thread.
		// performSelectorOnMainThread retains the image until the selector runs.
		[[NSApplication sharedApplication]
			performSelectorOnMainThread:@selector(setApplicationIconImage:)
							 withObject:image
						  waitUntilDone:NO];
		[image release];
	}
	return 1;
}
