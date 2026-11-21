package main

import (
	"io"
	"runtime"

	"github.com/Insadem/multi-roblox-macos/internal/deeplink"
	"github.com/Insadem/multi-roblox-macos/internal/macosapp"
	"github.com/Insadem/multi-roblox-macos/internal/robloxapp"
	"github.com/Insadem/multi-roblox-macos/internal/syncbreaker"
	"github.com/Insadem/multi-roblox-macos/internal/urlhandler"
)

// AppKit must run on the process's main thread. Locking here, during init,
// pins the main goroutine to it; otherwise the cgo calls below can let the
// scheduler move main() to another thread and NSApplication traps on launch.
func init() {
	runtime.LockOSThread()
}

func main() {
	urlhandler.Set("com.insadem.multiroblox", "roblox-player")
	s := urlhandler.Check(urlhandler.ROBLOX_BUNDLE_IDENTIFIER, "roblox-player")
	if s {
		macosapp.Notification("can't set new deeplink")
		return
	}

	urls := deeplink.Handler() // registers the Apple Event handler; keep it on the main thread

	cc := make(chan io.Closer)
	func(cc chan<- io.Closer) {
		go func() {
			for url := range urls {
				copy, err := robloxapp.NewCopy()
				if err != nil {
					macosapp.Notification("can't make new app copy: " + err.Error())
					continue
				}

				go func() { // not the best solution, but it's alright =). cost of 1 goroutine is ~8kb ram.
					cc <- copy
				}()

				// Stop Roblox's updater from closing the clients already running,
				// then remove the single-instance semaphore so the copy starts.
				robloxapp.PrepareLaunch()
				syncbreaker.Break()

				if err := copy.Launch(url); err != nil {
					macosapp.Notification("can't open roblox app copy: " + err.Error())
					continue
				}
			}
		}()
	}(cc)

	macosapp.Run(func() { // handle app termination here
		urlhandler.Set(urlhandler.ROBLOX_BUNDLE_IDENTIFIER, "roblox-player")

		close(cc)
		for c := range cc {
			c.Close()
		}
	})
}
