package plgn

import (
	"fmt"
)

type PluginIdentifier struct {
	PluginName    string
	PluginVersion string
}

func (plugin *PluginIdentifier) String() string {
	return fmt.Sprintf("%v (v%v)", plugin.PluginName, plugin.PluginVersion)
}

func NewPluginIdentifier(name string, version string) *PluginIdentifier {
	return &PluginIdentifier{
		PluginName:    name,
		PluginVersion: version,
	}
}
