package reseer

import (
    "fmt"
    "testing"
    "time"
)

//------------------------------------------------------------
// Tests
//------------------------------------------------------------

// Runs for N minutes giving time to experiment
// by changing files and directories inside 
// the test data directory.
func TestWatch(t *testing.T) {
    fmt.Println("\n\n\n\n")

    s, err := New(
        "data_test/rsrc_tracker.csv",
        []string{
            "data_test/dir-a",
            "data_test/dir-b",
        },
        onChange,
    )

    if err != nil {
        t.Errorf("Error in reseer: %v", err)
        return
    }

    // Keep running for some time
    var dur time.Duration = 3 * 60 * time.Second
    fmt.Println("Letting run for", dur)
    time.Sleep(dur)
    fmt.Println("...run time elapsed, finishing...")
    s.Stop()
    fmt.Println("FINISHED.")
}

func onChange(ver string) {
    fmt.Println("===> Client callback on version change to:", ver)
}
