package file

import (
	"testing"

	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m MockFileInfo) Name() string {
	return m.name
}

func (m MockFileInfo) Size() int64 {
	return m.size
}

func (m MockFileInfo) Mode() os.FileMode {
	return m.mode
}

func (m MockFileInfo) ModTime() time.Time {
	return m.modTime
}

func (m MockFileInfo) IsDir() bool {
	return m.isDir
}

func (m MockFileInfo) Sys() interface{} {
	return nil
}

func MockReadDir(dirname string) (files []os.FileInfo, err error) {
	mfi := MockFileInfo{
		name:    "abc.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   false,
	}
	mfi2 := MockFileInfo{
		name:    "abc2.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   false,
	}
	files = append(files, mfi)
	files = append(files, mfi2)
	return
}

func MockExistsPath(path string) (bool, error) {
	return true, nil
}

func TestGatherer(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readDirFunc = MockReadDir
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": false}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	items, err := g.Run(c, mockConfig)
	assert.Nil(t, err, "Unexpected error thrown")
	assert.Equal(t, 1, len(items), "Custom Gather should return 1 inventory type data.")
}
