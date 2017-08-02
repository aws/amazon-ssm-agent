package proc

import (
	"testing"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"src/github.com/stretchr/testify/assert"
)

var contextMock = context.NewMockDefault()
var logger = log.NewMockLog()

//TODO remove this unittest once we goes to production, it does not verify anything, just for prototyping purposes
/*
	According the https://en.wikipedia.org/wiki/Process_state#Terminated Wait() needs to be called to clear up process table
 	During the test on Darwin, as soon as Wait() is called, "sleep" disappeared from process table. No matter Release() is called or not it still showed up as zombie in the process table.
 	This can potentially cause resource leaking. Golang doc: https://golang.org/src/os/exec.go?s=3227:3260#L90 indicate Release() can replace Wait() when necessary, the testing result however showed otherwise.
*/
func TestStartProcessAndRelease(t *testing.T) {
	proc := NewOSProcess(contextMock)
	pid, err := proc.StartProcess("/bin/sleep", []string{"100"})
	assert.NoError(t, err)
	logger.Info("process launched: ", pid)
	logger.Info("releasing the attached process...")
	err = proc.Release()
	assert.NoError(t, err)
	time.Sleep(20 * time.Second)
}

func TestStartProcessAndKill(t *testing.T) {
	proc := NewOSProcess(contextMock)
	pid, err := proc.StartProcess("/bin/sleep", []string{"1"})
	assert.NoError(t, err)
	logger.Info("process launched: ", pid)
	logger.Info("releasing the attached process...")

	err = proc.Release()
	assert.NoError(t, err)
	logger.Info("killing the attached process...")
	err = proc.Kill()
	assert.NoError(t, err)
	time.Sleep(50 * time.Second)

}
