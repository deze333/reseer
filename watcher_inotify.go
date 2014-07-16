package reseer

import (
	"fmt"
	"time"

	"code.google.com/p/go.exp/inotify"
)

//------------------------------------------------------------
// Watcher inotify
//------------------------------------------------------------

type WatchInotify struct {
	callback func(string)
	count    int
	nextId   int
	watchers map[int]*inotify.Watcher
	dirs     map[int]string
	timer    *time.Timer
	damper   time.Duration
}

//------------------------------------------------------------
// Watcher inotify new
//------------------------------------------------------------

func newInotify(callback func(string), dirs []string) (w *WatchInotify, err error) {
	ds := NewDirScanner(dirs)
	count := len(ds.AllDirs)

	if count == 0 {
		err = fmt.Errorf("[reseer.inotify] FAIL: No directories to watch")
		return
	}

	// Create new watch
	w = &WatchInotify{
		callback: callback,
		damper:   3 * time.Second,
	}

	err = w.startRetry(ds.AllDirs, dirs)
	fmt.Println("[reseer.inotify] Started OK, watched directories =", len(w.watchers))
	return
}

//------------------------------------------------------------
// Watcher inotify functions
//------------------------------------------------------------

// Close all watchers.
func (w *WatchInotify) stop() {
	for i := 0; i < len(w.watchers); i++ {
		watcher := w.watchers[i]
		if watcher != nil {
			watcher.Close()
			w.watchers[i] = nil
		}
	}
}

// Starts new inotify watchers for given set of directories.
func (w *WatchInotify) startRetry(dirs, coreDirs []string) (err error) {

	if err = w.start(dirs); err == nil {
		return
	}

	// Make a few retries
	for i := 0; i < 3; i++ {
		fmt.Println("[reseer.inotify] WARNING: Will attempt to restart again in a few seconds\n\tdue to error:", err)
		time.Sleep(2 * time.Second)
		err = w.start(NewDirScanner(coreDirs).AllDirs)
		if err == nil {
			return
		}
	}
	return
}

// Starts new inotify watchers for given set of directories.
func (w *WatchInotify) start(dirs []string) (err error) {
	count := len(dirs)
	w.nextId = 0
	w.dirs = make(map[int]string, count)
	w.watchers = make(map[int]*inotify.Watcher, count)

	// Add each directory to a separate watcher
	for _, dir := range dirs {
		err = w.add(dir,
			inotify.IN_CREATE|inotify.IN_MOVE|inotify.IN_DELETE|inotify.IN_CLOSE_WRITE)
		if err != nil {
			w.stop()
			return
		}
	}
	return
}

// Checks if number of directories changed.
// If yes, restarts watchers for new set of directories.
func (w *WatchInotify) review(dirs []string) (err error) {
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
	// Give some time to inotify to execute (not critical)
	// Restart with a new set of directories
	w.stop()
	time.Sleep(2 * time.Second)
	w.startRetry(ds.AllDirs, dirs)
	fmt.Println("[reseer.inotify] Restarted OK, n =", len(w.watchers))
	return
}

// Adds watcher for one directory.
func (w *WatchInotify) add(dir string, flags uint32) (err error) {
	var watcher *inotify.Watcher
	watcher, err = inotify.NewWatcher()
	if err != nil {
		return err
	}

	if flags != 0 {
		err = watcher.AddWatch(dir, flags)
	} else {
		err = watcher.Watch(dir)
	}

	if err != nil {
		return err
	}

	w.watchers[w.nextId] = watcher
	w.dirs[w.nextId] = dir
	go w.watch(w.nextId)

	w.nextId++
	return
}

// Go routine that waits for an change event and notifies via callback
func (w *WatchInotify) watch(id int) {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("[reseer.inotify] Abnormal exit")
		}
	}()

	watcher := w.watchers[id]
	dir := w.dirs[id]
	if watcher == nil || dir == "" {
		return
	}
	//fmt.Println("[reseer.inotify] Watching dir:", dir)
	for {
		select {
		case ev := <-watcher.Event:
			if ev == nil {
				fmt.Println("[reseer.inotify] Closed for dir:", dir)
				return
			}
			//fmt.Println("[reseer.inotify] Change in:", ev)
			w.scheduleCallback(id, dir)

		case err := <-watcher.Error:
			fmt.Println("[reseer.inotify] ERROR:", err)
		}
	}
}

// Event damper.
// Calls callback only after duration elapsed since last change event.
// Prevents from lots of events firing at the same time.
func (w *WatchInotify) scheduleCallback(id int, dir string) {

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
