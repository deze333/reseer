package reseer

import (
	"fmt"
	"time"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

//------------------------------------------------------------
// Watcher fsnotify
//------------------------------------------------------------
type WatchFsnotify struct {
	watcher  *fsnotify.Watcher
	callback func(string)
	dirs     []string
	timer    *time.Timer
	damper   time.Duration
}

//------------------------------------------------------------
// Watcher fsnotify new
//------------------------------------------------------------

func newFsnotify(callback func(string), dirs []string) (w *WatchFsnotify, err error) {

	ds := NewDirScanner(dirs)
	count := len(ds.AllDirs)

	if count == 0 {
		err = fmt.Errorf("[reseer.fsnotify] FAIL: No directories to watch")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		err = fmt.Errorf("[reseer.fsnotify] FAIL: Error creating watcher")
		return
	}

	// Create new watch
	w = &WatchFsnotify{
		watcher:  watcher,
		callback: callback,
		damper:   3 * time.Second,
	}

	err = w.startRetry(ds.AllDirs, dirs)
	fmt.Println("[reseer.fsnotify] Started OK, watched directories =", len(ds.AllDirs))
	return
}

//------------------------------------------------------------
// Watcher fsnotify functions
//------------------------------------------------------------

// Close all watchers.
func (w *WatchFsnotify) stop() {

	if w.watcher != nil {
		w.watcher.Close()
		w.watcher = nil
	}
}

// Starts new fsnotify watchers for given set of directories.
func (w *WatchFsnotify) startRetry(dirs, coreDirs []string) (err error) {

	if err = w.start(dirs); err == nil {
		return
	}

	// Make a few retries
	for i := 0; i < 3; i++ {
		fmt.Println("[reseer.fsnotify] WARNING: Will attempt to restart again in a few seconds\n\tdue to error:", err)
		time.Sleep(2 * time.Second)
		err = w.start(NewDirScanner(coreDirs).AllDirs)
		if err == nil {
			return
		}
	}
	return
}

// Starts new fsnotify watchers for given set of directories.
func (w *WatchFsnotify) start(dirs []string) (err error) {

	// Start watching function
	go w.watch()

	// Add each directory to a separate watcher
	for _, dir := range dirs {
		err = w.watcher.Add(dir)
		if err != nil {
			return err
		}
		w.dirs = append(w.dirs, dir)
		if err != nil {
			w.stop()
			return
		}
	}
	return
}

// Checks if number of directories changed.
// If yes, restarts watchers for new set of directories.
func (w *WatchFsnotify) review(dirs []string) (err error) {

	ds := NewDirScanner(dirs)
	updatedDirs := ds.AllDirs
	currentDirs := w.dirs

	// Compare directories for both count and matching names
	if len(updatedDirs) == len(currentDirs) {
		isSame := true
		for _, dir := range updatedDirs {
			found := false
			for _, cdir := range currentDirs {
				if dir == cdir {
					found = true
					break
				}
			}
			if !found {
				isSame = false
				break
			}
		}
		if isSame {
			return
		}
	}

	//fmt.Println("Dirs CHANGED. New =", len(updatedDirs), "Old =", len(currentDirs))

	// Remove watch on existing directories
	// and set watch on updated set of directories
	for _, dir := range currentDirs {
		w.watcher.Remove(dir)
	}

	for _, dir := range updatedDirs {
		w.watcher.Add(dir)
	}

	return
}

// Go routine that waits for an change event and notifies via callback
func (w *WatchFsnotify) watch() {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("[reseer.fsnotify] Abnormal exit")
		}
	}()

	for {
		select {
		case event := <-w.watcher.Events:
			dir, _ := filepath.Split(event.Name)
			//fmt.Println("[reseer.fsnotify] EVENT: Change in dir:", dir)
			w.scheduleCallback(dir)

		case err := <-w.watcher.Errors:
			fmt.Println("[reseer.fsnotify] EVENT: ERROR:", err)
		}
	}
	fmt.Println("[reseer.fsnotify] Watcher function has exited")
}

// Event damper.
// Calls callback only after duration elapsed since last change event.
// Prevents from lots of events firing at the same time.
func (w *WatchFsnotify) scheduleCallback(dir string) {

	// Schedule a new callback via time hasn't been scheduled yet
	if w.timer == nil {
		w.timer = time.NewTimer(w.damper)

		// Create function waiting for timer event
		go func() {
			<-w.timer.C
			w.callback(dir)
			w.timer = nil
		}()

	} else {
		// Restart existing timer
		w.timer.Reset(w.damper)
	}
}
