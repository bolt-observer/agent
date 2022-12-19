package filter

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	utils "github.com/bolt-observer/go_common/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

type FileFilter struct {
	Filter
	WhitelistFilePath string
	Mutex             sync.Mutex
}

func (f *FileFilter) Reload() error {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	r := regexp.MustCompile(`\s*#.*$`)

	readFile, err := os.Open(f.WhitelistFilePath)

	if err != nil {
		return err
	}

	defer readFile.Close()

	f.nodeIdWhitelist = make(map[string]struct{})
	f.chanIdWhitelist = make(map[uint64]struct{})

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
			f.nodeIdWhitelist[line] = struct{}{}
		} else {
			val, err := strconv.ParseUint(line, 10, 64)
			if err != nil {
				glog.Warningf("Invalid line %s", line)
				continue
			}

			f.chanIdWhitelist[val] = struct{}{}
		}
	}

	glog.V(3).Infof("Filter reloaded")

	return nil
}

func NewFilterFromFile(ctx context.Context, filePath string, interval time.Duration) (FilterInterface, error) {
	var ticker *time.Ticker

	f := &FileFilter{
		WhitelistFilePath: filePath,
	}

	err := f.Reload()
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if interval > 0*time.Second {
		ticker = time.NewTicker(interval)
	}

	err = watcher.Add(filePath)
	if err != nil {
		return nil, err
	}

	go func(watcher *fsnotify.Watcher) {
		for {
			select {
			case <-ticker.C:
				f.Reload()
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
				if ticker != nil {
					ticker.Stop()
				}
				if watcher != nil {
					watcher.Close()
				}
				return
			}
		}
	}(watcher)

	return f, nil
}

func (f *FileFilter) AllowPubKey(id string) bool {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
	_, ok := f.nodeIdWhitelist[id]

	return ok
}

func (f *FileFilter) AllowChanId(id uint64) bool {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	_, ok := f.chanIdWhitelist[id]

	return ok
}
