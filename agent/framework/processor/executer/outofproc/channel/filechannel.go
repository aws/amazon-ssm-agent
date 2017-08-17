package channel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"strconv"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/fsnotify/fsnotify"
)

const (
	defaultFileCreateMode = 0750
	//exclusive flag works on windows, while 660 blocks others access to the file
	defaultFileWriteMode = os.ModeExclusive | 0660
)

//TODO use jsonutil instead of json
type fileWatcherChannel struct {
	logger        log.T
	path          string
	tmpPath       string
	onMessageChan chan string
	closeChan     chan bool
	mode          Mode
	counter       int
	recvCounter   int
	startTime     string
	watcher       *fsnotify.Watcher
	sendChan      chan string
}

/*
	Construct file channel, a file channel is identified by its unique name
 	At master mode, file && dirs will be destroyed at close time
	At slave mode, detach from the directory and leave
*/
func NewFileWatcherChannel(logger log.T, mode Mode) *fileWatcherChannel {
	return &fileWatcherChannel{
		logger: logger,
		mode:   mode,
	}
}

func createIfNotExist(dir string) (err error) {
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		//configure it to be not accessible by others
		err = os.MkdirAll(dir, defaultFileCreateMode)
	}
	return
}

func (ch *fileWatcherChannel) Open(name string) (err error) {
	log := ch.logger
	ch.path = name
	ch.tmpPath = path.Join(name, "tmp")
	ch.counter = 0
	curTime := time.Now()
	ch.startTime = fmt.Sprintf("%04d%02d%02d%02d%02d%02d", curTime.Year(), curTime.Month(), curTime.Day(), curTime.Hour(), curTime.Minute(), curTime.Second())
	//TODO if client is RunAs, server needs to grant client user R/W access respectively
	if err = createIfNotExist(ch.path); err != nil {
		log.Errorf("failed to create directory: %v", err)
		os.RemoveAll(ch.path)
		//if err occurs, the channel is not healthy anymore, should return false
		return
	}
	if err = createIfNotExist(ch.tmpPath); err != nil {
		log.Errorf("failed to create directory: %v", err)
		os.RemoveAll(ch.path)
		//if err occurs, the channel is not healthy anymore, should return false
		return
	}

	//buffered channel in order not to block listener
	ch.onMessageChan = make(chan string, defaultChannelBufferSize)
	//signal the message poller to stop
	ch.closeChan = make(chan bool, defaultChannelBufferSize)
	ch.sendChan = make(chan string, defaultChannelBufferSize)

	//initialize receiving counter
	ch.recvCounter = 0

	//start file watcher and monitor the directory
	if ch.watcher, err = fsnotify.NewWatcher(); err != nil {
		log.Errorf("filewatcher listener encountered error when start watcher: %v", err)
		return
	}

	if err = ch.watcher.Add(ch.path); err != nil {
		log.Errorf("filewatcher listener encountered error when add watch: %v", err)
		return
	}
	go ch.watch()
	return nil
}

func (ch *fileWatcherChannel) Send(rawJson string) error {
	//TODO deal with buffer channel overflow
	ch.sendChan <- rawJson
	return nil
}

/*
	drop a file in the destination path with the file name as sequence id
	the file is first named as tmp, then quickly renamed to guarantee atomicity
	sequence id format: {mode}-{command start time}-{counter} , squence id is guaranteed to be ascending order

*/
func (ch *fileWatcherChannel) send(rawJson string) error {
	log := ch.logger
	sequenceID := fmt.Sprintf("%v-%s-%03d", ch.mode, ch.startTime, ch.counter)
	filepath := path.Join(ch.path, sequenceID)
	tmp_filepath := path.Join(ch.tmpPath, sequenceID)
	//ensure sync exclusive write
	//TODO sender need to handle the case when connection is closed halfway
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

func (ch *fileWatcherChannel) GetMessageChannel() chan string {
	//TODO check for connected?
	return ch.onMessageChan
}

func (ch *fileWatcherChannel) Close() {
	log := ch.logger
	log.Debugf("channel %v requested close", ch.path)
	ch.closeChan <- true
	//master should remove the dir at close
	if ch.mode == ModeMaster {
		log.Debug("master removing directory...")
		os.RemoveAll(ch.path)
	}
	close(ch.closeChan)
	close(ch.sendChan)
	close(ch.onMessageChan)
	return
}

//parse the counter out of the sequence id, return -1 if parsing fails
func parseSequenceCounter(filepath string) int {
	_, name := path.Split(filepath)
	parts := strings.Split(name, "-")
	if len(parts) != 3 {
		return -1
	}
	counter, err := strconv.ParseInt(parts[2], 10, 64)
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
			}
		}
	}
}

func (ch *fileWatcherChannel) isReadable(filename string) bool {
	return !strings.Contains(filename, string(ch.mode)) && !strings.Contains(filename, "tmp")
}

//read and remove a given file
func (ch *fileWatcherChannel) consume(filepath string) {
	log := ch.logger
	log.Debugf("consuming message under path: %v", filepath)
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Errorf("message %v failed to read: %v \n", filepath, err)
		return

	}

	//remove the consumed file
	os.Remove(filepath)
	//update the recvcounter
	ch.recvCounter = parseSequenceCounter(filepath)
	//TODO handle buffered channel queue overflow
	ch.onMessageChan <- string(buf)
}

// we need to launch watcher receiver in another go routine, putting watcher.Close() and the receiver in same go routine can
// end up dead lock
// make sure this go routine not leaking
func (ch *fileWatcherChannel) watch() {
	log := ch.logger
	log.Debugf("%v listener started on path: %v", ch.mode, ch.path)
	//drain all the current messages in the dir
	ch.consumeAll()
	for {
		select {
		//TODO implementing onError handling
		case message := <-ch.sendChan:
			if err := ch.send(message); err != nil {
				log.Errorf("failed to send message: %v", err)
			}
		case <-ch.closeChan:
			log.Debug("closing file watcher listener...")
			ch.watcher.Close()
			return
		case event, ok := <-ch.watcher.Events:
			if !ok {
				log.Debug("fileWatcher closed")
				return
			}
			log.Debug("received event: ", event.String())
			if event.Op&fsnotify.Create == fsnotify.Create && ch.isReadable(event.Name) {
				//if the receiving counter is as expected, consume that message
				//otherwise, read the entire directory in sorted order, sender assures sending order
				if parseSequenceCounter(event.Name) == ch.recvCounter+1 {
					log.Debug("received out-of-order file update, polling the dir to reorder")
					ch.consume(event.Name)
				} else {
					ch.consumeAll()
				}
			}
		case err := <-ch.watcher.Errors:
			log.Errorf("file watcher error:", err)
		}
	}

}
