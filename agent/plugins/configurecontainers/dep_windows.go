package configurecontainers

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"golang.org/x/sys/windows/registry"
)

type dependencies_windows interface {
	dependencies
	RegistryOpenKey(k registry.Key, path string, access uint32) (registry.Key, error)
	RegistryKeySetDWordValue(key registry.Key, name string, value uint32) error
	RegistryKeyGetStringValue(key registry.Key, name string) (val string, valtype uint32, err error)
	FileutilUncompress(src, dest string) error
}

var dep dependencies_windows = &deps{}

func (deps) RegistryOpenKey(k registry.Key, path string, access uint32) (registry.Key, error) {
	return registry.OpenKey(k, path, access)
}
func (deps) RegistryKeySetDWordValue(key registry.Key, name string, value uint32) error {
	return key.SetDWordValue(name, value)
}
func (deps) RegistryKeyGetStringValue(key registry.Key, name string) (val string, valtype uint32, err error) {
	return key.GetStringValue(name)
}

func (deps) FileutilUncompress(src, dest string) error {
	return fileutil.Uncompress(src, dest)
}
