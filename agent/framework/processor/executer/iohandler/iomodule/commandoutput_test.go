package iomodule

import (
	"testing"

	"io"

	"sync"

	"bytes"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

// TestCommandOuput tests the CommandOutput module
func TestCommandOuput(t *testing.T) {
	type testCase struct {
		PipeInput string
		FileInput string
		Output    string
	}

	testCases := []testCase{
		{
			PipeInput: "Test input text.",
			FileInput: "",
			Output:    "Test input text.",
		},
		{
			PipeInput: " A sample \ninput text.",
			FileInput: "Test input text.",
			Output:    "Test input text. A sample \ninput text.",
		},
		{
			PipeInput: " Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non. " +
				"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. " +
				"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, " +
				"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat" +
				" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer" +
				" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, " +
				"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae " +
				"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu " +
				"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
			FileInput: "Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, " +
				"eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam " +
				"voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione " +
				"voluptatem sequi nesciunt. Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, " +
				"adipisci velit, sed quia non numquam eius modi tempora incidunt ut labore et dolore magnam aliquam quaerat" +
				" voluptatem. Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam," +
				" nisi ut aliquid ex ea commodi consequatur? Quis autem vel eum iure reprehenderit qui in ea voluptate " +
				"velit esse quam nihil molestiae consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla " +
				"pariatur? At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium " +
				"voluptatum deleniti atque corrupti quos dolores et quas molestias excepturi sint occaecati cupiditate" +
				" non provident, similique sunt in culpa qui officia deserunt mollitia animi, id est " +
				"laborum et dolorum fuga. Et harum quidem rerum facilis est et expedita distinctio." +
				" Nam libero tempore, cum soluta nobis est eligendi optio cumque nihil impedit quo minus id quod maxime placeat facere, ",
			Output: "Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, " +
				"eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam " +
				"voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione " +
				"voluptatem sequi nesciunt. Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, " +
				"adipisci velit, sed quia non numquam eius modi tempora incidunt ut labore et dolore magnam aliquam quaerat" +
				" voluptatem. Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam," +
				" nisi ut aliquid ex ea commodi consequatur? Quis autem vel eum iure reprehenderit qui in ea voluptate " +
				"velit esse quam nihil molestiae consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla " +
				"pariatur? At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium " +
				"voluptatum deleniti atque corrupti quos dolores et quas molestias excepturi sint occaecati cupiditate" +
				" non provident, similique sunt in culpa qui officia deserunt mollitia animi, id est " +
				"laborum et dolorum fuga. Et harum quidem rerum facilis est et expedita distinctio." +
				" Nam libero tempore, cum soluta nobis est eligendi optio cumque nihil impedit quo minus id quod maxime placeat facere " +
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non. " +
				"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. " +
				"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, " +
				"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat" +
				" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer" +
				" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, " +
				"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae " +
				"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu " +
				"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
		},
		{
			PipeInput: " \b5Ὂg̀9! ℃ᾭG",
			FileInput: "For a file \b5Ὂg̀9! ℃ᾭG \b5Ὂg̀9! ℃ᾭG",
			Output:    "For a file \b5Ὂg̀9! ℃ᾭG \b5Ὂg̀9! ℃ᾭG",
		},
	}

	i := 0
	for _, testCase := range testCases {
		pipeTestCase := testCase.PipeInput
		fileTestCase := testCase.FileInput
		output := testCase.Output
		stdout := testFileCommandOutput(pipeTestCase, fileTestCase, 30)
		if len(output) > 30 {
			assert.Equal(t, output[:30], stdout)
		} else {
			assert.Equal(t, output, stdout)
		}
		i++
	}
}

func testFileCommandOutput(pipeTestCase string, fileTestCase string, limit int) string {
	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	var stdout string
	stdoutConsole := CommandOutput{
		OutputLimit:            limit,
		OutputString:           &stdout,
		FileName:               "file",
		OrchestrationDirectory: "orchestrationDir",
	}

	wg.Add(1)
	var fileBuffer bytes.Buffer
	if len(fileTestCase) > limit {
		fileBuffer = *bytes.NewBufferString(fileTestCase[:limit])
	} else {
		fileBuffer = *bytes.NewBufferString(fileTestCase)
	}

	go func() {
		defer wg.Done()
		stdoutConsole.ReadPipeAndFile(logger, r, fileBuffer)
	}()

	w.Write([]byte(pipeTestCase))
	w.Close()
	wg.Wait()
	return stdout

}
