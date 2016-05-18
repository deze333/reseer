package reseer

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
)

//------------------------------------------------------------
// Watcher fsnotify
//------------------------------------------------------------
type WatchFsnotify struct {
	callback func(string)
	count    int
	nextId   int
	watchers map[int]*fsnotify.Watcher
	dirs     map[int]string
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

	// Create new watch
	w = &WatchFsnotify{
		callback: callback,
		damper:   3 * time.Second,
	}

	err = w.startRetry(ds.AllDirs, dirs)
	fmt.Println("[reseer.fsnotify] Started OK, watched directories =", len(w.watchers))
	return
}

//------------------------------------------------------------
// Watcher fsnotify functions
//------------------------------------------------------------

// Close all watchers.
func (w *WatchFsnotify) stop() {
	for i := 0; i < len(w.watchers); i++ {
		watcher := w.watchers[i]
		if watcher != nil {
			watcher.Close()
			w.watchers[i] = nil
		}
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
	count := len(dirs)
	w.nextId = 0
	w.dirs = make(map[int]string, count)
	w.watchers = make(map[int]*fsnotify.Watcher, count)

	// Add each directory to a separate watcher
	for _, dir := range dirs {
		err = w.add(dir)
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
	count := len(ds.AllDirs)

	// Compare directories for both count and matching names
	if count == len(w.dirs) {
		isSame := true
		for _, dir := range ds.AllDirs {
			found := false
			for _, cdir := range w.dirs {
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

	// Stop all existing
	// Give some time to fsnotify to execute (not critical)
	// Restart with a new set of directories
	w.stop()
	time.Sleep(2 * time.Second)
	w.startRetry(ds.AllDirs, dirs)
	fmt.Println("[reseer.fsnotify] Restarted OK, n =", len(w.watchers))
	return
}

// Adds watcher for one directory.
func (w *WatchFsnotify) add(dir string) (err error) {
	var watcher *fsnotify.Watcher
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(dir)

	if err != nil {
		return err
	}

	w.watchers[w.nextId] = watcher
	w.dirs[w.nextId] = dir
	go w.watch(watcher, dir, w.nextId)

	w.nextId++
	return
}

// Go routine that waits for an change event and notifies via callback
func (w *WatchFsnotify) watch(watcher *fsnotify.Watcher, dir string, id int) {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("[reseer.fsnotify] Abnormal exit")
		}
	}()

	//watcher := w.watchers[id]
	//dir := w.dirs[id]
	if watcher == nil || dir == "" {
		return
	}

	//fmt.Println("[reseer.fsnotify] Watching dir:", dir)
WatchLoop:
	for {
		select {
		case _, ok := <-watcher.Events:
			if !ok {
				break WatchLoop
			}
			//fmt.Println("[reseer.fsnotify] Change in:", ev)
			w.scheduleCallback(id, dir)

		case err, ok := <-watcher.Errors:
			if !ok {
				break WatchLoop
			}
			if err != nil {
				fmt.Println("[reseer.fsnotify] ERROR:", err)
			}
		}
	}
	fmt.Println("[reseer.fsnotify] Closed for dir:", dir)
}

// Event damper.
// Calls callback only after duration elapsed since last change event.
// Prevents from lots of events firing at the same time.
func (w *WatchFsnotify) scheduleCallback(id int, dir string) {

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
