// To run test with provided example directory:
// go test
// To run test with custom directory:
// go test -dir /path/to/my/directory

package reseer

import (
    "fmt"
    "testing"
	"flag"
)

//------------------------------------------------------------
// Tests
//------------------------------------------------------------

// Runs for N minutes giving time to experiment
// by changing files and directories inside 
// the test data directory.
func TestWatch(t *testing.T) {
    fmt.Println("\n\n\n\n")

	// Define which directory to watch
	var watchDirs []string
	if len(paramWatchDir) != 0 {
		watchDirs = []string{ paramWatchDir }

		fmt.Println("Watching  directory:", paramWatchDir)

		scanner := NewDirScanner(watchDirs)
		fmt.Println("All watched subdirectories count:", len(scanner.AllDirs))

		fmt.Println("\n\n")

	} else {
        watchDirs = []string{
            "data_test/dir-a",
            "data_test/dir-b",
        }
	}

	// Create new watcher
    s, err := New(
        "data_test/rsrc_tracker.csv",
		watchDirs,
        onChange,
    )

    if err != nil {
        t.Errorf("Error in reseer: %v", err)
        return
    }

    defer s.Stop()

    // Keep running for some time
    fmt.Println("Watching until Ctrl-C...")
	done := make(chan bool)
	<-done
}

func onChange(ver string) {
    fmt.Println("===> Client callback on version change to:", ver)
}

// Optional parameters for test runs.
var paramWatchDir string

// Parses optional flags.
func init() {
	flag.StringVar(&paramWatchDir, "dir", "", "directory to watch")
	flag.Parse()
}
