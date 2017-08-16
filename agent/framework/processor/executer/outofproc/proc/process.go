package proc

import "time"

//ProcessController encapsulate a os.Process object and provide limited access to the subprocesss
type ProcessController interface {
	//start a new process, with its I/O attached to the current pparent process
	StartProcess(name string, argv []string) (pid int, err error)
	//release the attached sub-process; if the sub-process is already detached, this call should be no-op
	Release() error
	//kill the enclosed process, no-op if the process non exists
	Kill() error
	//given pid and process create time, return true is the process is still active (no Z)
	Find(pid int, createTime time.Time) bool
}
