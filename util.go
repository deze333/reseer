package reseer

import (
    "fmt"
	"os"
	"path/filepath"
)

//------------------------------------------------------------
// Directory scanner
//------------------------------------------------------------

type DirScanner struct {
    Dirs []string
    AllDirs []string
}

func NewDirScanner(dirs []string) (s *DirScanner) {
    s = &DirScanner{}

	// Only allow existing directories
	s.Dirs = []string{}
	for _, dir := range dirs {
        if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Println("[reseer.scanner] Skipping not existing dir:", dir)
			continue
		}
		s.Dirs = append(s.Dirs, dir)
	}

    // Scan
    s.scan()
    return
}

func (s *DirScanner) scan() {
	s.AllDirs = []string{}
	for _, dir := range s.Dirs {
        _ = filepath.Walk(dir, s.scanDir)
	}
}

func (s *DirScanner) scanDir(path string, f os.FileInfo, err error) error {
	if f.IsDir() {
		s.AllDirs = append(s.AllDirs, path)
	}
	return nil
}

