package filter

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	utils "github.com/bolt-observer/go_common/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

// FileFilter struct
type FileFilter struct {
	Filter
	WhitelistFilePath string
	Mutex             sync.Mutex
	DefaultOptions    Options
}

// Reload from file
func (f *FileFilter) Reload() error {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	r := regexp.MustCompile(`\s*#.*$`)

	readFile, err := os.Open(f.WhitelistFilePath)

	if err != nil {
		return err
	}

	defer readFile.Close()

	f.nodeIDWhitelist = make(map[string]struct{})
	f.chanIDWhitelist = make(map[uint64]struct{})
	f.Options = f.DefaultOptions

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		line := strings.Trim(fileScanner.Text(), " ")

		if strings.HasPrefix("#", line) {
			continue
		}

		line = strings.Trim(r.ReplaceAllString(line, ""), " \t\r\n")
		if line == "" {
			continue
		}

		if utils.ValidatePubkey(line) {
			f.nodeIDWhitelist[line] = struct{}{}
		} else if strings.ToLower(line) == "private" {
			f.Options |= AllowAllPrivate
		} else if strings.ToLower(line) == "public" {
			f.Options |= AllowAllPublic
		} else {
			val, err := strconv.ParseUint(line, 10, 64)
			if err != nil {
				glog.Warningf("Invalid line %s", line)
				continue
			}

			f.chanIDWhitelist[val] = struct{}{}
		}
	}

	glog.V(3).Infof("Filter reloaded")

	return nil
}

// NewFilterFromFile create new FileFilter
func NewFilterFromFile(ctx context.Context, filePath string, options Options) (FilteringInterface, error) {
	f := &FileFilter{
		WhitelistFilePath: filePath,
	}

	f.DefaultOptions = options

	err := f.Reload()
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(filePath)
	if err != nil {
		return nil, err
	}

	go func(watcher *fsnotify.Watcher) {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					continue
				}

				f.Reload()

				if event.Op&fsnotify.Rename == fsnotify.Rename || event.Op&fsnotify.Remove == fsnotify.Remove {
					// Happens when you save the changes via text editor
					err := watcher.Add(event.Name)
					if err != nil {
						glog.Warningf("Watcher error %v\n", err)
					}
				}
			case err := <-watcher.Errors:
				glog.Warningf("Watcher error %v\n", err)
			case <-ctx.Done():
				if watcher != nil {
					watcher.Close()
				}
				return
			}
		}
	}(watcher)

	return f, nil
}

// AllowPubKey checks whether pubkey is allowed
func (f *FileFilter) AllowPubKey(id string) bool {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
	_, ok := f.nodeIDWhitelist[id]

	return ok
}

// AllowChanID checks short channel id
func (f *FileFilter) AllowChanID(id uint64) bool {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	_, ok := f.chanIDWhitelist[id]

	return ok
}

// AllowSpecial is used to allow all private/public chans
func (f *FileFilter) AllowSpecial(private bool) bool {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	if private {
		return f.Options&AllowAllPrivate == AllowAllPrivate
	}
	return f.Options&AllowAllPublic == AllowAllPublic
}
