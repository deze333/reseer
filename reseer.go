package reseer

import (
    "errors"
    "fmt"
    "os"
    "bufio"
    "path/filepath"
    "strconv"
    "time"
    "encoding/csv"
)

var (
    ErrDiff = errors.New("DIFF")
)

//------------------------------------------------------------
// Seer
//------------------------------------------------------------

type Seer struct {
    isValid bool
    Filename string
    version int
    versionTxt string
    dirs []string
    entries [][]string
    wInotify *WatchInotify
    clientCallback func(string)
}

//------------------------------------------------------------
// Seer New
//------------------------------------------------------------

func New(filename string, dirs []string, cb func(string)) (s *Seer, err error) {

    // Validate
    if filename == "" {
        err = fmt.Errorf("[reseer] ERROR-ABORT: Tracking file must be provided")
        return
    }

    if len(dirs) == 0 {
        err = fmt.Errorf("[reseer] ERROR-ABORT: One or more directories must be provided")
        return
    }

    for _, dir := range dirs {
        if _, err = os.Stat(dir); os.IsNotExist(err) {
            err = fmt.Errorf("[reseer] ERROR-ABORT: Directory not found: %v", dir)
            return
        }
    }

    // Create new
    s = &Seer{
        isValid: true,
        Filename: filename,
        dirs: dirs,
        clientCallback: cb,
    }

    // Start watching
    if err = s.start(); err != nil {
        s = nil
    }
    return
}

//------------------------------------------------------------
// Seer functions
//------------------------------------------------------------

func (s *Seer) VersionTxt() (vtxt string) {
    return s.versionTxt
}

func (s *Seer) Stop() {
    if s.wInotify != nil {
        fmt.Println("[reseer] Stop request, waiting for inotify stop...")
        s.wInotify.stop()
        time.Sleep(1 * time.Second)
    }
}


func (s *Seer) start() (err error) {
    if _, err = os.Stat(s.Filename); os.IsNotExist(err) {
        // Tracking file doesn't exist, reset it
        if err = s.resetTracker(); err != nil {
            fmt.Println(err)
            return
        }
    } else {
        // Tracking file exists
        // Load tracking file
        // Scan all directories
        // Compare with tracking file
        // If any diffs create and save new tracker
        fmt.Println("[reseer] Loading existing tracker")
        if err = s.loadTracker(); err != nil {
            // Tracking file corrupt, reset it
            fmt.Println(err)
            if err = s.resetTracker(); err != nil {
                return
            }
        }
        // Compare current dirs with tracked
        if err = s.compareDirs(); err != nil {
            if err.Error() != ErrDiff.Error() {
                return
            }
            fmt.Println("[reseer] Tracker state is DIFFERENT from directories, updating")
            err = nil
            // Increment version
            s.setVersion(s.version + 1)
            // Create new tracker
            s.initEntries()
            if err = s.scanDirs(); err != nil {
                return
            }
            if err = s.saveTracker(); err != nil {
                return
            }
        }
    }

    // Start watching discovered dirs for changes
    s.wInotify, err = newInotify(s.onChange, s.dirs)
    if err != nil {
        err = fmt.Errorf("[reseer] ERROR-ABORT: Can't create inotify watcher: %v", err)
        return
    }

    return
}

// Reset tracker to current directories' state.
func (s *Seer) resetTracker() (err error) {
    // Reset version number
    // Scan all directories
    // Create tracking file
    // Save tracking file
    fmt.Println("[reseer] Resetting tracker")
    s.setVersion(1)
    s.initEntries()
    if err = s.scanDirs(); err != nil {
        return
    }
    if err = s.saveTracker(); err != nil {
        return
    }
    return
}

// Callback function on a change in watched directories.
// Called from goroutine run by watcher.
func (s *Seer) onChange(dir string) {
    var err error

    // Overview:
    // Compare for changes
    // If changes:
    // - Increment version
    // - Scan all directories
    // - Create tracking file
    // - Save tracking file

    if err = s.compareDirs(); err != nil {
        if err.Error() == ErrDiff.Error() {
            fmt.Println("[reseer] Version changed to", s.version + 1, "due to change in:", dir)
            s.setVersion(s.version + 1)
        }
    }

    // New files may have been added
    // Rescan all and update tracking file
    s.initEntries()
    if err = s.scanDirs(); err != nil {
        panic(fmt.Sprintf("[reseer] ERROR-ABORT: onChange: can't scan dirs, err: %v", err))
    }
    if err = s.saveTracker(); err != nil {
        panic(fmt.Sprintf("[reseer] ERROR-ABORT: onChange: can't save tracking file, err: %v", err))
    }

    // Ask watcher to review directories
    err = s.wInotify.review(s.dirs)
    if err != nil {
        panic(fmt.Sprintf("[reseer] ERROR-ABORT: onChange: restarting inotify watcher, err: %v", err))
    }

    // Fire client callback
    if s.clientCallback != nil {
        s.clientCallback(s.versionTxt)
    }
}


// Sets version.
func (s *Seer) setVersion(v int) {
    s.version = v
    s.versionTxt = strconv.Itoa(v)
}

func (s *Seer) initEntries() {
    s.entries = [][]string{}
    s.entries = append(s.entries, []string{"V", s.versionTxt, "reseer"})
}


func (s *Seer) scanDirs() (err error) {
    for _, dir := range s.dirs {
        err = filepath.Walk(dir, s.scanDir)
    }
    return
}


func (s *Seer) scanDir(path string, f os.FileInfo, err error) error {
    // File or directory?
    if f.IsDir() {
        return nil
    }

    // Ensure time stamp encoding/decoding works
    t1 := f.ModTime()
    t1t := strconv.FormatInt(f.ModTime().UnixNano(), 10)
    t1i, _ := strconv.ParseInt(t1t, 10, 64)
    t2 := time.Unix(0, t1i)
    if !t1.Equal(t2) {
        return fmt.Errorf("[reseer] BUG: Time encoding/decoding failed for file: %v", path)
    }
    s.entries = append(s.entries, []string{"F", path, t1t})
    return nil
}

func (s *Seer) compareDirs() (err error) {
    for _, dir := range s.dirs {
        err = filepath.Walk(dir, s.compareDir)
        if err != nil {
            return 
        }
    }
    return
}

func (s *Seer) compareDir(path string, f os.FileInfo, err error) error {
    if ! f.IsDir() {
        entry := s.findDir(path)
        if entry == nil {
            return nil
        }

        // If name found, compare modification times
        if strconv.FormatInt(f.ModTime().UnixNano(), 10) != entry[2] {
            return ErrDiff
        } else {
        }
    }
    return nil
}


func (s *Seer) findDir(path string) (entry []string) {
    for _, entry = range s.entries {
        if(entry[1] == path) {
            return entry
        }
    }
    return nil
}


func (s *Seer) saveTracker() error {
    file, err := os.Create(s.Filename)
    if err != nil {
        return err
    }
    defer file.Close()
    return csv.NewWriter(bufio.NewWriter(file)).WriteAll(s.entries)
}


func (s *Seer) loadTracker() error {
    file, err := os.Open(s.Filename)
    if err != nil {
        return err
    }
    defer file.Close()
    s.entries, err = csv.NewReader(bufio.NewReader(file)).ReadAll()
    if err != nil {
        return err
    }

    // Parse version
    if s.entries == nil || len(s.entries) == 0 {
        return fmt.Errorf("[reseer] WARNING: Tracker file empty")
    }

    if s.entries[0][0] != "V" {
        return fmt.Errorf("[reseer] WARNING: Tracker file malformed. First line must have version, instead: %v", s.entries[0])
    }

    vs := s.entries[0][1]
    s.version, err = strconv.Atoi(vs)
    if err != nil {
        return fmt.Errorf("[reseer] WARNING: Tracker file malformed. First line must have version integer number in the second place: %v", s.entries[0])
    }
    s.versionTxt = vs

    fmt.Println("LOADED TRACKER AT V:", s.version, s.versionTxt)
    return err
}
