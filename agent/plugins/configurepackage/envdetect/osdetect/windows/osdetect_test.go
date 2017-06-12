package windows

import (
	"testing"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestDetectPkgManager(t *testing.T) {
	d := Detector{}
	result, err := d.DetectPkgManager("", "", "") // parameters only matter for linux

	assert.NoError(t, err)
	assert.Equal(t, c.PackageManagerWindows, result)
}

func TestDetectInitSystem(t *testing.T) {
	d := Detector{}
	result, err := d.DetectInitSystem()

	assert.NoError(t, err)
	assert.Equal(t, c.InitWindows, result)
}
