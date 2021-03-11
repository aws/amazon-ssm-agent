package iomodule

import (
	"io"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

// TestCommandOuput tests the CommandOutput module
func TestCommandOuput(t *testing.T) {

	// TestInputCases is a list of strings which we test multi-writer on.
	context := context.NewMockDefault()
	var TestInputCases = [...]string{
		"Test input text.",
		"A sample \ninput text.",
		"\b5Ὂg̀9! ℃ᾭG",
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non. " +
			"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. " +
			"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, " +
			"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat" +
			" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer" +
			" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, " +
			"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae " +
			"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu " +
			"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
	}

	i := 0
	for _, testCase := range TestInputCases {
		stdout := testFileCommandOutput(context, testCase, i)
		assert.Equal(t, testCase, stdout)
		i++
	}
}

func testFileCommandOutput(context context.T, pipeTestCase string, i int) string {
	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	var stdout string
	stdoutConsole := CommandOutput{
		OutputString:           &stdout,
		FileName:               "file" + strconv.Itoa(i),
		OrchestrationDirectory: "testdata",
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		stdoutConsole.Read(context, r, appconfig.SuccessExitCode)
	}()

	w.Write([]byte(pipeTestCase))
	w.Close()
	wg.Wait()
	return stdout

}
