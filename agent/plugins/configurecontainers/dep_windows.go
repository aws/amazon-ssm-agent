package configurecontainers

import "github.com/aws/amazon-ssm-agent/agent/fileutil"

type dependencies_windows interface {
	dependencies
	FileutilUncompress(src, dest string) error
}

var dep dependencies_windows = &deps{}

func (deps) FileutilUncompress(src, dest string) error {
	return fileutil.Uncompress(src, dest)
}
