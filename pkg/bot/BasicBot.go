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

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/plgn"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

const (
	initialNlSize     = 11
	initialModuleSize = 11
	pathToPluginFiles = "./plugins"
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

type Remotable interface {
	// Adds a remote location
	AddRemote(host ...string)
	// Remove a remote location
	RemoveRemote(host ...string)
}

type Pluggable interface {
	// ability to add a pluging to add an active plugin
	AddActivePlugin(plugin *plgn.PluginIdentifier)
	// ability to remove an active plugin
	RemoveActivePlugin(plugin *plgn.PluginIdentifier)
	// ability to spawn a bot
	spawnBot()
	// ability to load a plugin
	loadPlugin(ident *plgn.PluginIdentifier) (*plugin.Plugin, error)
	// load a plugin from the filesystem
	loadFsLocalPlugin(path string) (*plugin.Plugin, error)
	// load required functions and variables from plugin
	loadFunctionsAndVariablesFromPlugin(plgn plugin.Plugin) (*PluginContract, error)
	// method to handle calls to run plugins
	handleRun(ctx actor.Context)
}

type Subscribable interface {
	notifySubscribers(ctx actor.Context, message interface{})
	handleSubscribe(ctx actor.Context, message *msg.Subscribe)
	handleUnsubscribe(ctx actor.Context, message *msg.Unsubscribe)
}

// Basic bot
type BasicBot interface {
	// getter and setter
	// list of neighboring bots
	AddPeer(pid ...*actor.PID)
	RemovePeer(pid ...*actor.PID)
	// a bot that is capable of spawning remote actors
	Remotable
	// ability to plug in functionality
	Pluggable
	// ability to subscribe/ unsubscribe to messages
	Subscribable
	// a bot is an actor so it must have a Receive method
	Receive(ctx actor.Context)
	// lifecycle methods
	handleStarted()
	handleStopped()
	handleStopping()
	// ability to spawn bots
	handleSpawn(ctx actor.Context)
	// handle notification that a bot was spawned
	handleSpawned(ctx actor.Context)
}
