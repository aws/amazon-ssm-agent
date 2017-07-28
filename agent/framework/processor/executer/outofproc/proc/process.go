package proc

//ProcessController encapsulate a os.Process object and provide limited access to the subprocesss
type ProcessController interface {
	//start a new process, with its I/O attached to the current pparent process
	StartProcess(name string, argv []string) (pid int, err error)
	//release the attached sub-process; if the sub-process is already detached, this call should be no-op
	Release() error
	//kill the enclosed process, no-op if the process non exists
	Kill() error
}
