package darwin

import (
	"fmt"
	"testing"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestPlatformDetect(t *testing.T) {
	data := []struct {
		input    string
		expected string
	}{
		{"10.12.3", "10.12.3"},
		{"asdf", "asdf"},
		{"", ""},
	}
	for _, m := range data {
		t.Run(fmt.Sprintf("%s in %s", m.input, m.expected), func(t *testing.T) {
			resultVersion := extractDarwinVersion([]byte(m.input))
			assert.Equal(t, m.expected, string(resultVersion))
		})
	}
}

func TestDetectInitSystem(t *testing.T) {
	d := Detector{}
	result, err := d.DetectInitSystem()

	assert.NoError(t, err)
	assert.Equal(t, c.InitLaunchd, result)
}

func TestDetectPkgManager(t *testing.T) {
	d := Detector{}
	result, err := d.DetectPkgManager("", "", "") // parameters only matter for linux

	assert.NoError(t, err)
	assert.Equal(t, c.PackageManagerMac, result)
}
