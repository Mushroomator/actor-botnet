package bot

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
	"net/url"
	"plugin"

	"github.com/Mushroomator/actor-bots/pkg/plgn"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

const (
	initialNlSize     = 11
	initialModuleSize = 11
	pathToPluginFiles = "../plugins"
)

var (
	defaultPluginRepoUrl = &url.URL{
		Scheme: "https",
		Host:   "go-plugin-repo.s3.eu-central-1.amazonaws.com",
	}
	logger = log.New(log.DefaultLevel, "[Bot]")
	//logger = log.Default()
)

type PluginContract struct {
	Receive func(bot *SimpleBot, ctx actor.Context)
}

// Basic bot
type BasicBot interface {
	// getter and setter
	// list of neighboring bots
	SetNl(nl []*actor.PID)
	Nl() []*actor.PID
	// list of plugins
	SetPlugins(plugins map[plgn.PluginIdentifier]interface{})
	Plugins() map[plgn.PluginIdentifier]interface{}
	// a bot is an actor so it must have a Receive method
	Receive(ctx actor.Context)
	// initialize the bot
	handleStarted()
	handleStopped()
	handleStopping()
	// ability to load a plugin
	loadPlugin(ident plgn.PluginIdentifier) (*plugin.Plugin, error)
	// load a plugin from the filesystem
	loadFsLocalPlugin(path string) (*plugin.Plugin, error)
	// load required functions and variables from plugin
	loadFunctionsAndVariablesFromPlugin(plgn plugin.Plugin) (*PluginContract, error)
}
