package filewatcherbasedipc

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"strconv"

	"errors"

	"regexp"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/cenkalti/backoff"
	"github.com/fsnotify/fsnotify"
)

const (
	defaultChannelBufferSize = 100

	defaultFileCreateMode = 0750
	//exclusive flag works on windows, while 660 blocks others access to the file
	defaultFileWriteMode = os.ModeExclusive | 0660

	consumeAttemptCount                = 3
	consumeRetryIntervalInMilliseconds = 20
)

var (
	osStatFn = os.Stat
)

//TODO add unittest
type fileWatcherChannel struct {
	logger        log.T
	path          string
	tmpPath       string
	onMessageChan chan string
	mode          Mode
	counter       int
	//the next expected message
	recvCounter              int
	startTime                string
	watcher                  *fsnotify.Watcher
	mu                       sync.RWMutex
	closed                   bool
	shouldDeleteAfterConsume bool
	shouldReadRetry          bool
}

//TODO make this constructor private
/*
	Create a file channel, a file channel is identified by its unique name
	name is the path where the watcher directory is created
 	Only Master channel has the privilege to remove the dir at close time
    shouldReadRetry - is this flag is set to true, it will use fileReadWithRetry function to read
*/
func NewFileWatcherChannel(logger log.T, mode Mode, name string, shouldReadRetry bool) (*fileWatcherChannel, error) {

	tmpPath := path.Join(name, "tmp")
	curTime := time.Now()
	//TODO if client is RunAs, server needs to grant client user R/W access respectively
	if err := createIfNotExist(name); err != nil {
		logger.Errorf("failed to create directory: %v", err)
		os.RemoveAll(name)
		//if err occurs, the channel is not healthy anymore, should return false
		return nil, err
	}
	if err := createIfNotExist(tmpPath); err != nil {
		logger.Errorf("failed to create directory: %v", err)
		os.RemoveAll(name)
		//if err occurs, the channel is not healthy anymore, should return false
		return nil, err
	}

	//buffered channel in order not to block listener
	onMessageChan := make(chan string, defaultChannelBufferSize)

	//start file watcher and monitor the directory
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Errorf("filewatcher listener encountered error when start watcher: %v", err)
		os.RemoveAll(name)
		return nil, err
	}

	if err = watcher.Add(name); err != nil {
		logger.Errorf("filewatcher listener encountered error when add watch: %v", err)
		os.RemoveAll(name)
		return nil, err
	}

	ch := &fileWatcherChannel{
		path:            name,
		tmpPath:         tmpPath,
		watcher:         watcher,
		onMessageChan:   onMessageChan,
		logger:          logger,
		mode:            mode,
		counter:         0,
		recvCounter:     0,
		shouldReadRetry: shouldReadRetry,
		startTime:       fmt.Sprintf("%04d%02d%02d%02d%02d%02d", curTime.Year(), curTime.Month(), curTime.Day(), curTime.Hour(), curTime.Minute(), curTime.Second()),
	}
	if ch.mode == ModeRespondent {
		ch.shouldDeleteAfterConsume = false
	} else {
		ch.shouldDeleteAfterConsume = true
	}
	go ch.watch()
	return ch, nil
}

func createIfNotExist(dir string) (err error) {
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		//configure it to be not accessible by others
		err = os.MkdirAll(dir, defaultFileCreateMode)
	}
	return
}

/*
	drop a file in the destination path with the file name as sequence id
	the file is first named as tmp, then quickly renamed to guarantee atomicity
	sequence id format: {mode}-{command start time}-{counter} , squence id is guaranteed to be ascending order

*/
func (ch *fileWatcherChannel) Send(rawJson string) error {
	if ch.closed {
		return errors.New("channel already closed")
	}
	log := ch.logger
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	sequenceID := fmt.Sprintf("%v-%s-%03d", ch.mode, ch.startTime, ch.counter)
	filepath := path.Join(ch.path, sequenceID)
	tmp_filepath := path.Join(ch.tmpPath, sequenceID)
	//ensure sync exclusive write
	if err := ioutil.WriteFile(tmp_filepath, []byte(rawJson), defaultFileWriteMode); err != nil {
		log.Errorf("write file %v encountered error: %v \n", tmp_filepath, err)
		return err
	}
	if err := os.Rename(tmp_filepath, filepath); err != nil {
		log.Errorf("send renaming file encountered error: %v", err)
		return err
	}
	//file successfully sent, increment counter
	ch.counter++
	return nil
}

func (ch *fileWatcherChannel) GetMessage() <-chan string {
	return ch.onMessageChan
}

func (ch *fileWatcherChannel) Destroy() {
	ch.Close()
	//only master can remove the dir at close
	if ch.mode == ModeMaster || ch.mode == ModeSurveyor {
		ch.logger.Debug("master removing directory...")
		if err := os.RemoveAll(ch.path); err != nil {
			ch.logger.Errorf("failed to remove directory %v : %v", ch.path, err)
		}
	}
}

// CleanupOwnModeFiles cleans up it own mode files
func (ch *fileWatcherChannel) CleanupOwnModeFiles() {
	ch.logger.Debugf("cleaning up all the messages under mode: %v", ch.mode)
	fileInfos, _ := ioutil.ReadDir(ch.path)
	if len(fileInfos) > 0 { // not needed in go. just a safety check
		for _, info := range fileInfos {
			name := info.Name()
			if ch.isFileFromSameMode(name) {
				ch.removeMessage(filepath.Join(ch.path, name))
			}
		}
	}
}

// GetPath returns IPC filepath
func (ch *fileWatcherChannel) GetPath() string {
	return ch.path
}

func (ch *fileWatcherChannel) removeMessage(filePath string) {
	var err error
	for attempt := 0; attempt < consumeAttemptCount; attempt++ {
		err = os.Remove(filePath)
		if err != nil {
			ch.logger.Debugf("message %v failed to remove (attempt %v): %v \n", filePath, attempt+1, err)
			time.Sleep(time.Duration(consumeRetryIntervalInMilliseconds) * time.Millisecond)
		} else {
			break
		}
	}
	if err != nil {
		ch.logger.Error("Error occurred while removing the IPC file: ", err.Error())
	}
}

// isFileFromSameMode checks whether file matches the current file mode or not
// also check for the file pattern mode-startTime-counter
func (ch *fileWatcherChannel) isFileFromSameMode(filename string) bool {
	matched, err := regexp.MatchString("[a-zA-Z]+-[0-9]+-[0-9]+", filename)
	if !matched || err != nil {
		return false
	}
	return strings.Contains(filename, string(ch.mode)) && !strings.Contains(filename, "tmp")
}

// Close a filechannel
// non-blocking call, drain the buffered messages and clear file watcher resources
func (ch *fileWatcherChannel) Close() {
	if ch.closed {
		return
	}
	log := ch.logger
	log.Infof("channel %v requested close", ch.path)
	//block other threads to call Send()
	ch.closed = true
	//read all the left over messages
	ch.consumeAll()
	// fsnotify.watch.close() could be a blocking call, we should offload them to a different go-routine
	go func() {
		defer func() {
			if msg := recover(); msg != nil {
				log.Errorf("closing file watcher panics: %v", msg)
			}
			close(ch.onMessageChan)
			log.Infof("channel %v closed", ch.path)
		}()
		//make sure the file watcher closed as well as the watch list is removed, otherwise can cause leak in ubuntu kernel
		ch.watcher.Remove(ch.path)
		ch.watcher.Close()
	}()

	return
}

//parse the counter out of the sequence id, return -1 if parsing fails
//counter is defined as the padding last element of - separated integer
//On windows, path.Base() does not work
func parseSequenceCounter(filepath string) int {
	_, name := path.Split(filepath)
	parts := strings.Split(name, "-")
	counter, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return -1
	}
	return int(counter)
}

//read all messages in the consuming dir, with order guarantees -- ioutil.ReadDir() sort by name, and name is the lexicographical ascending sequence id.
//filter out its own sent messages and tmp messages
func (ch *fileWatcherChannel) consumeAll() {
	ch.logger.Debug("consuming all the messages under: ", ch.path)
	fileInfos, _ := ioutil.ReadDir(ch.path)
	if len(fileInfos) > 0 {
		for _, info := range fileInfos {
			name := info.Name()
			if ch.isReadable(name) {
				ch.consume(path.Join(ch.path, name))
			} else {
				ch.logger.Debugf("IPC file not readable: %s", name)
			}
		}
	}
}

//TODO add unittest
func (ch *fileWatcherChannel) isReadable(filename string) bool {
	matched, err := regexp.MatchString("[a-zA-Z]+-[0-9]+-[0-9]+", filename)
	if !matched || err != nil {
		return false
	}
	return !strings.Contains(filename, string(ch.mode)) && !strings.Contains(filename, "tmp")
}

//read and remove a given file
func (ch *fileWatcherChannel) consume(filepath string) {
	log := ch.logger
	log.Debugf("consuming message under path: %v", filepath)

	var buf []byte
	var err error
	if ch.shouldReadRetry {
		buf, err = fileReadWithRetry(filepath)
	} else {
		buf, err = fileRead(log, filepath)
	}
	if err != nil {
		log.Errorf("message %v failed to read: %v \n", filepath, err)
		return
	}
	if ch.shouldDeleteAfterConsume {
		//remove the consumed IPC file and log error message when there is an exception in os.Remove()
		ch.removeMessage(filepath)
	}

	//update the recvcounter
	ch.recvCounter = parseSequenceCounter(filepath) + 1
	//TODO handle buffered channel queue overflow
	ch.onMessageChan <- string(buf)
}

func fileRead(logger log.T, filepath string) (buf []byte, err error) {
	for attempt := 0; attempt < consumeAttemptCount; attempt++ {
		//On windows rename does not guarantee atomic access: https://github.com/golang/go/issues/8914
		//In exclusive mode we have, this read will for sure fail when it's locked by the other end
		buf, err = ioutil.ReadFile(filepath)
		if err != nil {
			logger.Debugf("message %v failed to read (attempt %v): %v \n", filepath, attempt+1, err)
			time.Sleep(time.Duration(consumeRetryIntervalInMilliseconds) * time.Millisecond)
		} else {
			break
		}
	}
	return
}

// TODO - create a new function for read using blocking file locks
func fileReadWithRetry(filepath string) (buf []byte, err error) {
	fileSize, err := getFileSize(filepath)
	if err != nil {
		return
	}

	exponentialBackOff, err := backoffconfig.GetExponentialBackoff(consumeRetryIntervalInMilliseconds*time.Millisecond, consumeAttemptCount)
	if err != nil {
		return
	}

	fileRead := func() (fileErr error) {
		buf, err = ioutil.ReadFile(filepath)
		if err != nil {
			fileErr = errors.New(fmt.Sprintf("error while consuming message: %v", err))
		}
		fileBufSize := int64(len(buf))
		if fileSize == 0 || fileBufSize == 0 || fileBufSize != fileSize {
			fileErr = errors.New(fmt.Sprintf("problem reading file - fileBufSize: %v, fileSize: %v", fileBufSize, fileSize))
		}
		return fileErr
	}

	err = backoff.Retry(fileRead, exponentialBackOff)
	return
}

func getFileSize(filepath string) (fileSize int64, err error) {
	var fileInfo os.FileInfo
	exponentialBackOff, err := backoffconfig.GetExponentialBackoff(consumeRetryIntervalInMilliseconds*time.Millisecond, consumeAttemptCount)
	if err != nil {
		return
	}

	fileStat := func() (err error) {
		fileInfo, err = osStatFn(filepath)
		return
	}
	err = backoff.Retry(fileStat, exponentialBackOff)
	if err != nil {
		return
	}
	fileSize = fileInfo.Size()
	return fileSize, err
}

// we need to launch watcher receiver in another go routine, putting watcher.Close() and the receiver in same go routine can
// end up dead lock
// make sure this go routine not leaking
func (ch *fileWatcherChannel) watch() {
	log := ch.logger
	defer log.Infof("%v listener stopped on path: %v", ch.mode, ch.path)

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("File watch panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	log.Infof("%v listener started on path: %v", ch.mode, ch.path)
	//drain all the current messages in the dir
	ch.consumeAll()
	for {
		select {
		case event, ok := <-ch.watcher.Events:
			if !ok {
				log.Debug("fileWatcher already closed")
				return
			}
			log.Debug("received event: ", event.String())
			if event.Op&fsnotify.Create == fsnotify.Create && ch.isReadable(event.Name) {
				//if the receiving counter is as expected, consume that message
				//otherwise, read the entire directory in sorted order, sender assures sending order
				if parseSequenceCounter(event.Name) == ch.recvCounter {
					ch.consume(event.Name)
				} else {
					log.Debug("received out-of-order file update, polling the dir to reorder")
					ch.consumeAll()
				}
			}
		case err := <-ch.watcher.Errors:
			if err != nil {
				log.Errorf("file watcher error: %v", err)
			}
		}
	}

}
