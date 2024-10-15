// Package lockfile handles pid file based locking.
// While a sync.Mutex helps against concurrency issues within a single process,
// this package is designed to help against concurrency issues between cooperating processes
// or serializing multiple invocations of the same process. You can also combine sync.Mutex
// with Lockfile in order to serialize an action between different goroutines in a single program
// and also multiple invocations of this program.
package lockfile

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)


type Lockfile interface {
	GetOwner() (*os.Process, error)
	ChangeOwner(int) error
	Unlock() error
	TryLock() error
	TryLockExpire(int64) error
	TryLockExpireWithRetry(int64) error
	ShouldRetry(error) bool
}

type lockfileImpl struct{
	lockPath string
	expirePath string
}

// TemporaryError is a type of error where a retry after a random amount of sleep should help to mitigate it.
type TemporaryError string

func (t TemporaryError) Error() string { return string(t) }

// Temporary returns always true.
// It exists, so you can detect it via
//	if te, ok := err.(interface{ Temporary() bool }); ok {
//		fmt.Println("I am a temporary error situation, so wait and retry")
//	}
func (t TemporaryError) Temporary() bool { return true }

// Various errors returned by this package
var (
	ErrBusy          = TemporaryError("Locked by other process")             // If you get this, retry after a short sleep might help
	ErrNotExist      = TemporaryError("Lockfile created, but doesn't exist") // If you get this, retry after a short sleep might help
	ErrNeedAbsPath   = errors.New("Lockfiles must be given as absolute path names")
	ErrInvalidPid    = errors.New("Lockfile contains invalid pid for system")
	ErrDeadOwner     = errors.New("Lockfile contains pid of process not existent on this system anymore")
	ErrRogueDeletion = errors.New("Lockfile owned by me has been removed unexpectedly")
)

// Assign method to global variables to allow unittest to override
var getUnixTime = getUnixTimeFunc

// New describes a new filename located at the given absolute path.
func New(path string) (Lockfile, error) {
	if !filepath.IsAbs(path) {
		return lockfileImpl {"", ""}, ErrNeedAbsPath
	}

	return lockfileImpl {path, path + ".expire"}, nil
}

// GetOwner returns who owns the lockfile.
func (l lockfileImpl) GetOwner() (*os.Process, error) {
	// Ok, see, if we have a stale lockfile here
	content, err := ioutil.ReadFile(l.lockPath)
	if err != nil {
		return nil, err
	}

	// try hard for pids. If no pid, the lockfile is junk anyway and we delete it.
	pid, err := scanPidLine(content)
	if err != nil {
		return nil, err
	}

	return getProcess(pid)
}

// ChangeOwner changes the pid in the lock file but only if the current pid owns the lockfile
func (l lockfileImpl) ChangeOwner(pid int) error {
	proc, err := l.GetOwner()

	// Only allow change pid if current process owns the lock
	switch err {
	default:
		// Other errors -> defensively fail and let caller handle this
		return err
	case nil:
		if proc.Pid != os.Getpid() {
			return ErrBusy
		}
	}

	// Make sure the process for the pid alive
	_, err = getProcess(pid)

	if err != nil {
		return err
	}

	return ioutil.WriteFile(l.lockPath, []byte(strconv.Itoa(pid)+"\n"), 0600)
}

// Unlock a lock again, if we owned it. Returns any error that happened during release of lock.
func (l lockfileImpl) Unlock() error {
	proc, err := l.GetOwner()
	switch err {
	case ErrInvalidPid, ErrDeadOwner:
		return ErrRogueDeletion
	case nil:
		if proc.Pid == os.Getpid() {
			// we really own it, so let's remove it.
			l.cleanUpLock()
			return nil
		}
		// Not owned by me, so don't delete it.
		return ErrRogueDeletion
	default:
		// This is an application error or system error.
		// So give a better error for logging here.
		if os.IsNotExist(err) {
			return ErrRogueDeletion
		}
		// Other errors -> defensively fail and let caller handle this
		return err
	}
}

// TryLock tries to own the lock.
// It Returns nil, if successful and and error describing the reason, it didn't work out.
// Please note, that existing lockfiles containing pids of dead processes
// and lockfiles containing no pid at all are simply deleted.
// If a expirelock is already owned by the process, the lock is chanced into a regular pid lock
func (l lockfileImpl) TryLock() error {
	// This has been checked by New already. If we trigger here,
	// the caller didn't use New and re-implemented it's functionality badly.
	// So panic, that he might find this easily during testing.
	if !filepath.IsAbs(l.lockPath) {
		panic(ErrNeedAbsPath)
	}

	tmplock, cleanup, err := makePidFile(l.lockPath, os.Getpid())
	if err != nil {
		return err
	}

	defer cleanup()

	// EEXIST and similar error codes, caught by os.IsExist, are intentionally ignored,
	// as it means that someone was faster creating this link
	// and ignoring this kind of error is part of the algorithm.
	// Then we will probably fail the pid owner check later, if this process is still alive.
	// We cannot ignore ALL errors, since failure to support hard links, disk full
	// as well as many other errors can happen to a filesystem operation
	// and we really want to abort on those.
	if err := os.Link(tmplock, l.lockPath); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	fiTmp, err := os.Lstat(tmplock)
	if err != nil {
		return err
	}

	fiLock, err := os.Lstat(l.lockPath)
	if err != nil {
		// tell user that a retry would be a good idea
		if os.IsNotExist(err) {
			return ErrNotExist
		}

		return err
	}

	// Success
	if os.SameFile(fiTmp, fiLock) && !l.isLockExpired() {
		return nil
	}

	proc, err := l.GetOwner()
	switch err {
	default:
		// Other errors -> defensively fail and let caller handle this
		return err
	case nil:
		if proc.Pid != os.Getpid() {
			if !l.isLockExpired() {
				return ErrBusy
			}
		}
	case ErrDeadOwner, ErrInvalidPid: // cases we can fix below
	}

	l.cleanUpLock()

	// now that the stale lockfile is gone, let's recurse
	return l.TryLock()
}

// TryLockExpire tries to own the lock while creating a expiration file as well.
// If expiration file already exists it updates the expiration with
func (l lockfileImpl) TryLockExpire(minutes int64) error {
	err := l.TryLock()

	// Unable to lock
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(l.expirePath, []byte(strconv.FormatInt(getUnixTime() + minutes * 60, 10) + "\n"), 0600)

	if err != nil {
		l.cleanUpLock()
	}

	return err
}

// TryLockExpireWithRetry tries to own the lock while creating a expiration file as well.
// This function retries when an error(except ErrBusy) thrown by TryLockExpire
func (l lockfileImpl) TryLockExpireWithRetry(minutes int64) (err error) {
	noOfRetries := 2
	for retryCount := 1; retryCount <= noOfRetries; retryCount++ {
		err = l.TryLockExpire(minutes)
		if l.ShouldRetry(err) {
			time.Sleep(100 * time.Millisecond)

			err = l.TryLockExpire(minutes)
			if l.ShouldRetry(err) {
				l.cleanUpLock()
			} else {
				return err
			}
		} else {
			return err
		}
	}

	return err
}

func (l lockfileImpl) cleanUpLock() {
	_ = os.Remove(l.lockPath)
	_ = os.Remove(l.expirePath)
}


func (l lockfileImpl) isLockExpired() bool {
	var expireTime int64
	timeNow := getUnixTime()

	content, err := ioutil.ReadFile(l.expirePath)

	if err != nil {
		// If the expire does not exist, the lock does not expire
		if os.IsNotExist(err) {
			return false
		}

		return true
	}

	// Any errors scanning timestamp is assumed as expiration
	if _, err := fmt.Sscanln(string(content), &expireTime); err != nil {
		return true
	}

	return expireTime < timeNow
}

func (l lockfileImpl) ShouldRetry(err error) bool {
	if err == nil || err == ErrBusy {
		return false
	}
	return true
}

func scanPidLine(content []byte) (int, error) {
	if len(content) == 0 {
		return 0, ErrInvalidPid
	}

	var pid int
	if _, err := fmt.Sscanln(string(content), &pid); err != nil {
		return 0, ErrInvalidPid
	}

	if pid <= 0 {
		return 0, ErrInvalidPid
	}

	return pid, nil
}

func makePidFile(name string, pid int) (tmpname string, cleanup func(), err error) {
	tmplock, err := ioutil.TempFile(filepath.Dir(name), filepath.Base(name)+".")
	if err != nil {
		return "", nil, err
	}

	cleanup = func() {
		_ = tmplock.Close()
		_ = os.Remove(tmplock.Name())
	}

	if _, err := io.WriteString(tmplock, fmt.Sprintf("%d\n", pid)); err != nil {
		cleanup() // Do cleanup here, so call doesn't have to.
		return "", nil, err
	}

	return tmplock.Name(), cleanup, nil
}

func getProcess(pid int) (proc *os.Process, err error){
	running, err := isRunning(pid)
	if err != nil {
		return nil, err
	}

	if running {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil, err
		}

		return proc, nil
	}

	return nil, ErrDeadOwner
}

func getUnixTimeFunc() int64 {
	return time.Now().Unix()
}