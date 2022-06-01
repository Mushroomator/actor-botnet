package plgn

// Copyright 2022 Thomas Pilz

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"fmt"
)

type Plugin interface {
	PluginName() string
	PluginVersion() string
	String() string
}

type PluginIdentifier struct {
	PlgnName    string
	PlgnVersion string
}

// comparator for plugins
func CmpPlugins(this, other interface{}) int {
	p1 := this.(Plugin)
	p2 := other.(Plugin)

	switch {
	case p1.PluginName() > p2.PluginName():
		return 1
	case p1.PluginName() < p2.PluginName():
		return -1
	case p1.PluginVersion() > p2.PluginVersion():
		return 1
	case p1.PluginVersion() < p2.PluginVersion():
		return -1
	default:
		return 0
	}
}

func (plugin *PluginIdentifier) String() string {
	return fmt.Sprintf("%v (v%v)", plugin.PlgnName, plugin.PlgnVersion)
}

func (plugin *PluginIdentifier) PluginName() string {
	return plugin.PlgnName
}

func (plugin *PluginIdentifier) PluginVersion() string {
	return plugin.PlgnVersion
}

func NewPluginIdentifier(name string, version string) *PluginIdentifier {
	return &PluginIdentifier{
		PlgnName:    name,
		PlgnVersion: version,
	}
}
