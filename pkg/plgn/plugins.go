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

type PluginIdentifier struct {
	PluginName    string
	PluginVersion string
}

// comparator for plugins
func CmpPlugins(this, other interface{}) int {
	p1 := this.(*PluginIdentifier)
	p2 := other.(*PluginIdentifier)

	switch {
	case p1.PluginName > p2.PluginName:
		return 1
	case p1.PluginName < p2.PluginName:
		return -1
	case p1.PluginVersion > p2.PluginVersion:
		return 1
	case p1.PluginVersion < p2.PluginVersion:
		return -1
	default:
		return 0
	}
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
