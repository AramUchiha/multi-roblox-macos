package robloxapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Insadem/multi-roblox-macos/pkg/ps"
)

// How long PrepareLaunch waits for a running Roblox installer to finish.
const installerWaitLimit = 90 * time.Second

// PrepareLaunch clears the Roblox updater state that would otherwise make a
// newly started client close every other running client.
//
// Roblox assigns release channels per account (some accounts get test channels
// such as "zbisect013" that serve a different build). A client signed in to
// such an account downloads that build in the background into
// ~/Library/Roblox/UnzippedBundle, records it in InstallerInfo.log and sets the
// shared channel preference. The next client to start then "finalizes" the
// install, and the installer terminates every process with Roblox's bundle ID,
// including our copies. Pinning the channel to production and removing the
// staged install keeps each new client on the build that is installed.
func PrepareLaunch() {
	waitForInstaller(installerWaitLimit)

	exec.Command("/usr/bin/defaults", "write",
		"com.roblox.RobloxPlayerChannel", "www.roblox.com", "production").Run()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	lib := filepath.Join(home, "Library", "Roblox")
	os.RemoveAll(filepath.Join(lib, "UnzippedBundle"))
	os.Remove(filepath.Join(lib, "RobloxPlayer.zip"))
	os.Remove(filepath.Join(lib, "RobloxPlayerInstaller", "InstallerInfo.log"))
}

// waitForInstaller blocks while a Roblox installer is running, up to limit, so a
// background download can't land between PrepareLaunch's cleanup and the launch.
func waitForInstaller(limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) && installerRunning() {
		time.Sleep(500 * time.Millisecond)
	}
}

func installerRunning() bool {
	procs, err := ps.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		// The kernel truncates process names, so match "RobloxPlayerInstaller" by prefix.
		if strings.HasPrefix(p.Executable(), "RobloxPlayerIns") {
			return true
		}
	}
	return false
}

// Launch starts the copy's player directly, handing it the roblox-player URL.
//
// Launching through LaunchServices (`open`) doesn't start a second client:
// Roblox's Info.plist sets LSMultipleInstancesProhibited, so LaunchServices
// passes the URL to a client that is already running (which then leaves its
// game) or refuses the launch. Starting the executable directly avoids that and
// leaves the copy's code signature untouched.
func (b Copy) Launch(url string) error {
	cmd := exec.Command(b.Path()+"/Contents/MacOS/RobloxPlayer", "-protocolString", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // don't share our process group
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap the process when it exits
	return nil
}
