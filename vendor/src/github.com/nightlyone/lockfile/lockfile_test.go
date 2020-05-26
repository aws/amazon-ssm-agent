package lockfile

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"testing"
)

func ExampleLockfile() {
	lock, err := New(filepath.Join(os.TempDir(), "lock.me.now.lck"))
	if err != nil {
		fmt.Printf("Cannot init lock. reason: %v", err)
		panic(err) // handle properly please!
	}

	// Error handling is essential, as we only try to get the lock.
	if err = lock.TryLock(); err != nil {
		fmt.Printf("Cannot lock %q, reason: %v", lock, err)
		panic(err) // handle properly please!
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			fmt.Printf("Cannot unlock %q, reason: %v", lock, err)
			panic(err) // handle properly please!
		}
	}()

	fmt.Println("Do stuff under lock")
	// Output: Do stuff under lock
}


func TestBasicLockUnlock(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		panic(err)
	}

	lf, err := New(path)
	if err != nil {
		t.Fail()
		fmt.Println("Error making lockfile: ", err)
		return
	}

	err = lf.TryLock()
	if err != nil {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	err = lf.Unlock()
	if err != nil {
		t.Fail()
		fmt.Println("Error unlocking lockfile: ", err)
		return
	}
}

func GetDeadPID() int {
	// I have no idea how windows handles large PIDs, or if they even exist.
	// So limit it to be less or equal to 4096 to be safe.
	const maxPid = 4095

	// limited iteration, so we finish one day
	seen := map[int]bool{}
	for len(seen) < maxPid {
		pid := rand.Intn(maxPid + 1) // see https://godoc.org/math/rand#Intn why
		if seen[pid] {
			continue
		}

		seen[pid] = true

		running, err := isRunning(pid)
		if err != nil {
			fmt.Println("Error checking PID: ", err)
			continue
		}

		if !running {
			return pid
		}
	}
	panic(fmt.Sprintf("all pids lower %d are used, cannot test this", maxPid))
}

func TestBusy(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}

	pid := os.Getppid()

	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0600); err != nil {
		t.Fatal(err)
		return
	}
	defer os.Remove(path)

	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}

	got := lf.TryLock()
	if got != ErrBusy {
		t.Fatalf("expected error %q, got %v", ErrBusy, got)
		return
	}
}

func TestRogueDeletion(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}
	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}
	err = lf.TryLock()
	if err != nil {
		t.Fatal(err)
		return
	}
	err = os.Remove(path)
	if err != nil {
		t.Fatal(err)
		return
	}

	got := lf.Unlock()
	if got != ErrRogueDeletion {
		t.Fatalf("unexpected error: %v", got)
		return
	}
}

func TestChangeOwner(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}

	pid := os.Getppid()

	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0600); err != nil {
		t.Fatal(err)
		return
	}
	defer os.Remove(path)


	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}

	deadPid := GetDeadPID()
	err = lf.ChangeOwner(deadPid)
	if err != ErrBusy {
		t.Fatal(err)
		return
	}

	os.Remove(path)

	err = lf.ChangeOwner(deadPid)
	if !os.IsNotExist(err) {
		t.Fatal(err)
		return
	}

	err = lf.TryLock()
	if err != nil {
		t.Fatal("Expected to be able to lock file")
	}

	err = lf.ChangeOwner(deadPid)
	if err != ErrDeadOwner {
		t.Fatal(err)
		return
	}

	err = lf.ChangeOwner(pid)
	if err != nil {
		t.Fatal(err)
		return
	}

	err = lf.TryLock()
	if err != ErrBusy {
		t.Fatal(err)
		return
	}
}

func TestRogueDeletionDeadPid(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}
	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}
	err = lf.TryLock()
	if err != nil {
		t.Fatal(err)
		return
	}

	pid := GetDeadPID()
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0666); err != nil {
		t.Fatal(err)
		return
	}

	defer os.Remove(path)

	err = lf.Unlock()
	if err != ErrRogueDeletion {
		t.Fatalf("unexpected error: %v", err)
		return
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			content, _ := ioutil.ReadFile(path)
			t.Fatalf("lockfile %q (%q) should not be deleted by us, if we didn't create it", path, content)
		}
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRemovesStaleLockOnDeadOwner(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}
	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}
	pid := GetDeadPID()
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0666); err != nil {
		t.Fatal(err)
		return
	}
	err = lf.TryLock()
	if err != nil {
		t.Fatal(err)
		return
	}

	if err := lf.Unlock(); err != nil {
		t.Fatal(err)
		return
	}
}

func TestInvalidPidLeadToReplacedLockfileAndSuccess(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}

	if err := ioutil.WriteFile(path, []byte("\n"), 0666); err != nil {
		t.Fatal(err)
		return
	}

	defer os.Remove(path)

	lf, err := New(path)
	if err != nil {
		t.Fatal(err)
		return
	}

	if err := lf.TryLock(); err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}

	// now check if file exists and contains the correct content
	got, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
		return
	}
	want := fmt.Sprintf("%d\n", os.Getpid())
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestScanPidLine(t *testing.T) {
	tests := [...]struct {
		input []byte
		pid   int
		xfail error
	}{
		{xfail: ErrInvalidPid},
		{input: []byte(""), xfail: ErrInvalidPid},
		{input: []byte("\n"), xfail: ErrInvalidPid},
		{input: []byte("-1\n"), xfail: ErrInvalidPid},
		{input: []byte("0\n"), xfail: ErrInvalidPid},
		{input: []byte("a\n"), xfail: ErrInvalidPid},
		{input: []byte("1\n"), pid: 1},
	}

	// test positive cases first
	for step, tc := range tests {
		if tc.xfail != nil {
			continue
		}

		got, err := scanPidLine(tc.input)
		if err != nil {
			t.Fatalf("%d: unexpected error %v", step, err)
		}

		if want := tc.pid; got != want {
			t.Errorf("%d: expected pid %d, got %d", step, want, got)
		}
	}

	// test negative cases now
	for step, tc := range tests {
		if tc.xfail == nil {
			continue
		}

		_, got := scanPidLine(tc.input)
		if want := tc.xfail; got != want {
			t.Errorf("%d: expected error %v, got %v", step, want, got)
		}
	}
}

func TestTryLockExpireWithExtension(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		panic(err)
	}

	lf := lockfileImpl{path, path + ".expire"}

	err = lf.TryLockExpire(30)
	if err != nil {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}
	defer lf.Unlock()

	if lf.isLockExpired() {
		t.Fail()
		fmt.Println("Error lock is expired")
		return
	}

	getUnixTime = func() int64 {
		return time.Now().Unix() + 30 * 60 + 1
	}

	if !lf.isLockExpired() {
		t.Fail()
		fmt.Println("Error lock shounld be expired2")
		return
	}

	// Should update to 60 minutes expiration
	getUnixTime = func() int64 {
		return time.Now().Unix()
	}
	err = lf.TryLockExpire(60)
	if err != nil {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	if lf.isLockExpired() {
		t.Fail()
		fmt.Println("Error lock should not be expired")
		return
	}

	getUnixTime = func() int64 {
		return time.Now().Unix() + 60 * 60 + 1
	}

	if !lf.isLockExpired() {
		t.Fail()
		fmt.Println("Error lock should be expired")
		return
	}
}

func TestLockOwnerByParentButExpires(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}

	pid := os.Getppid()

	// Lockfile
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0600); err != nil {
		t.Fatal(err)
		return
	}
	defer os.Remove(path)

	// expire in 1 second
	if err := ioutil.WriteFile(path + ".expire", []byte(strconv.FormatInt(getUnixTime() + 1, 10) + "\n"), 0600); err != nil {
		t.Fatal(err)
		return
	}
	defer os.Remove(path + ".expire")

	lf, err := New(path)
	if err != nil {
		t.Fail()
		fmt.Println("Error making lockfile: ", err)
		return
	}

	err = lf.TryLockExpire(30)
	if err != ErrBusy {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	err = lf.TryLock()
	if err != ErrBusy {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	time.Sleep(2 * time.Second)

	err = lf.TryLock()
	if err != nil {
		t.Fatal(err)
		return
	}

	lf.Unlock()
}

func TestTryLockExpireWhenOwned(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}

	lf := lockfileImpl{path, path + ".expire"}
	defer lf.Unlock()

	err = lf.TryLockExpire(30)
	if err != nil {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	err = lf.TryLockExpire(60)
	if err != nil {
		t.Fail()
		fmt.Println("Error locking lockfile: ", err)
		return
	}

	newExpireTime := time.Now().Unix() + 60 * 60 - 2
	var expireTime int64

	content, err := ioutil.ReadFile(lf.expirePath)
	_, _ = fmt.Sscanln(string(content), &expireTime)

	if expireTime < newExpireTime {
		t.Fail()
		fmt.Println("Failed to update expiration with second TryLockExpire")
		return
	}
}

func TestShouldRetry(t *testing.T) {
	path, err := filepath.Abs("test_lockfile.pid")
	if err != nil {
		t.Fatal(err)
		return
	}
	lf, err := New(path)

	if lf.ShouldRetry(nil) {
		t.Fail()
		fmt.Println("nil Should not retry")
		return
	}

	if !lf.ShouldRetry(ErrBusy) {
		t.Fail()
		fmt.Println("ErrBusy Should retry")
		return
	}

	if !lf.ShouldRetry(ErrNotExist) {
		t.Fail()
		fmt.Println("ErrNotExist Should retry")
		return
	}

	if lf.ShouldRetry(ErrNeedAbsPath) {
		t.Fail()
		fmt.Println("ErrNeedAbsPath Should not retry")
		return
	}

	if lf.ShouldRetry(ErrInvalidPid) {
		t.Fail()
		fmt.Println("ErrInvalidPid Should not retry")
		return
	}

	if lf.ShouldRetry(ErrDeadOwner) {
		t.Fail()
		fmt.Println("ErrDeadOwner Should not retry")
		return
	}

	if lf.ShouldRetry(ErrRogueDeletion) {
		t.Fail()
		fmt.Println("ErrRogueDeletion Should not retry")
		return
	}

	if lf.ShouldRetry(fmt.Errorf("Some other error")) {
		t.Fail()
		fmt.Println("Errors should not retry")
		return
	}

}
