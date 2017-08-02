package proc

import (
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
)

//ProcessController encapsulate a os.Process object and provide limited access to the subprocesss
//fork does not apply in multi-thread scenario, we have to start a new executable
type ProcessController interface {
	//start a new process, with its I/O attached to the current parent process
	//TODO remove the name argument, the path to the executable should be a constant
	StartProcess(name string, argv []string) (pid int, err error)
	//TODO we need to make sure not causing resource leak if Wait() is not called
	//release the attached sub-process; if the sub-process is already detached, this call should be no-op
	Release() error
	//kill the enclosed process, no-op if the process non exists
	Kill() error
	//given pid and process create time, return true is the process is still active (no Z)
	Find(pid int, createTime time.Time) bool
}

type OSProcess struct {
	process  *os.Process
	attached bool
	context  context.T
}

func NewOSProcess(ctx context.T) *OSProcess {
	return &OSProcess{
		attached: false,
		context:  ctx.With("[OSProcessController]"),
	}
}

func (p *OSProcess) StartProcess(name string, argv []string) (pid int, err error) {
	log := p.context.Log()
	var procAttr os.ProcAttr
	//TODO do we need to set env and dir in ProcAttr?
	//TODO substitute program name
	if p.process, err = os.StartProcess(name, argv, &procAttr); err != nil {
		log.Errorf("start process: &v encountered error : %v", name, err)
		return
	}
	pid = p.process.Pid
	p.attached = true
	return
}

func (p *OSProcess) Release() error {
	if p.attached {
		p.context.Log().Debug("Releasing os process...")
		p.attached = false
		return p.process.Release()
	}
	return nil
}

func (p *OSProcess) Kill() error {
	if p.attached {
		p.context.Log().Debug("Killing os process...")
		p.attached = false
		return p.Kill()
	}
	return nil
}
