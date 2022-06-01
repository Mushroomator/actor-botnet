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
	"errors"
	"fmt"
	"net/url"
	"plugin"
	"strings"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/configuration"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/plgn"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
)

var (
	logger = log.New(log.DefaultLevel, "[Bot]")
)

type FinishedFunc func()

// Specification/ Contract which a plugin must obey to
type PluginContract struct {
	OnActivated   func(bot *Bot, ctx actor.Context, plugin plgn.Plugin)
	OnDeactivated func(bot *Bot, ctx actor.Context, plugin plgn.Plugin)
	Receive       func(bot *Bot, ctx actor.Context, plugin plgn.Plugin, finished FinishedFunc)
}

// Holds remote locations (bot nodes) to spawn bots at
type Remotable interface {
	// Get remotes
	Remotes() sets.Set
	// Set remotes
	SetRemotes(remotes sets.Set)
	// Get peers
	// Adds a remote location
	AddRemote(host ...string)
	// Remove a remote location
	RemoveRemote(host ...string)
	// Get peers
	Peers(pid *actor.PID) *actor.PIDSet
	// Set peers
	SetPeers(peers *actor.PIDSet)
	// Add peer
	AddPeer(pid ...*actor.PID)
	// Remove peer
	RemovePeer(pid ...*actor.PID)
	// Spawn a bot on a given node
	SpawnBot(ctx actor.Context, host string, port int) (*actor.PID, error)
	// Kill a bot
	KillBot(ctx actor.Context, pid *actor.PID)
	// Handle *msg.Spawn
	handleSpawn(ctx actor.Context, message *msg.Spawn)
	// Handle *msg.Spawned
	handleSpawned(ctx actor.Context, message *msg.Spawned)
	// Handle *msg.Created
	handleCreated(ctx actor.Context, message *msg.Created)
}

// Ability to load plugins at runtime
type Pluggable interface {
	// Get logger for plugins
	PluginLogger() *log.Logger
	// Get active plugins
	ActivePlugins() []*plgn.PluginIdentifier
	// Add a plugin to active plugins
	AddActivePlugin(plugin *plgn.PluginIdentifier)
	// Remove a plugin from active plugins
	RemoveActivePlugin(plugin *plgn.PluginIdentifier)
	// Handle *msg.LoadPlugin message
	handleLoadPlugin(ctx actor.Context, message *msg.LoadPlugin)
	// Handle *msg.UnloadPlugin message
	handleUnloadPlugin(ctx actor.Context, message *msg.UnloadPlugin)
	// Load a plugin. Loads a plugin file which is obtained from a source (e.g. local filesystem, remote repo, peer etc.)
	loadPlugin(ident *plgn.PluginIdentifier) error
	// Loads a plugin file from one of the sources
	loadPluginFile(ident plgn.PluginIdentifier) (*plugin.Plugin, error)
	// Loads a plugin file from a source
	getPluginFromSource(ident plgn.PluginIdentifier, dest string) error
	// load a plugin from the filesystem
	loadFsLocalPlugin(path string) (*plugin.Plugin, error)
	// load required functions and variables from plugin
	loadFunctionsAndVariablesFromPlugin(goPlugin plugin.Plugin) (*PluginContract, error)
	// method to handle calls to run plugins
	handleRun(ctx actor.Context)
	// cleanup Plugin files
	cleanupPlugins()
}

// Handler methods for lifecycle event
type LifetimeObservable interface {
	// Handle *actor.Started
	handleStarted(ctx actor.Context)
	// Handle *actor.Terminated
	handleStopping(ctx actor.Context)
	// Handle *actor.Terminated
	handleStopped(ctx actor.Context)
	// Handle *actor.Terminated
	handleTerminated(ctx actor.Context, message *actor.Terminated)
}

// Central, pluggable repository to obtain plugins from
type PluggableRepository interface {
	// remote repository url
	RemoteRepoUrl() string
	// getter and setter
	SetRemoteRepoUrl(url string)
}

// Ability to subsribe/ unsubsribe to events
type Subscribable interface {
	// Get subscribers
	Subscribers() map[msg.MessageType]*actor.PIDSet
	// Adds a new subscriber to for a certain messaget type
	// If no message type is specified, a subscription for all messages is created
	AddSubscriber(subscriber *actor.PID, messageTypes ...msg.MessageType)
	// Handle *msg.Unsubscribe message
	RemoveSubscriber(unsubscriber *actor.PID, messageTypes ...msg.MessageType)
	// Notify subscribers about an event
	NotifySubscribers(ctx actor.Context, message interface{})
	// Handle *msg.Subsribe message
	handleSubscribe(ctx actor.Context, message *msg.Subscribe)
	// Handle *msg.Unsubscribe message
	handleUnsubscribe(ctx actor.Context, message *msg.Unsubscribe)
}

// Basic bot
type BasicBot interface {
	// a bot that is capable of spawning remote actors
	Remotable
	// ability to plug in functionality
	Pluggable
	// ability to subscribe/ unsubscribe to messages
	Subscribable
	// a bot must have an repository from which to download plugins
	PluggableRepository
	// a bot is an actor so it must have a Receive method
	Receive(ctx actor.Context)
	// a bot's lifecycle should be observable
	LifetimeObservable
}

type Remote struct {
	Host string
	Port int
}

// comparator for plugins
func CmpRemote(this, other interface{}) int {
	p1 := this.(*Remote)
	p2 := other.(*Remote)

	remote1 := fmt.Sprintf("%v:%v", p1.Host, p1.Port)
	remote2 := fmt.Sprintf("%v:%v", p2.Host, p2.Port)
	return strings.Compare(remote1, remote2)
}

type Bot struct {
	// list of neigboring bots
	peers *actor.PIDSet
	// list of loadedPlugins this bot has
	loadedPlugins map[plgn.Plugin]*PluginContract
	activePlugins *treeset.Set
	remotes       sets.Set
	remoter       *remote.Remote
	messageTypes  map[string]msg.MessageType
	pluginLogger  *log.Logger
	subscribers   map[msg.MessageType]*actor.PIDSet
	// plugin repository
	pluginRepoUrl *url.URL
}

// Create a new simple bot
func NewSimpleBot(remoter *remote.Remote) *Bot {
	if remoter == nil {
		panic(errors.New("Remoter must be specified"))
	}
	initSubsribers := func(msgTypes []msg.MessageType) map[msg.MessageType]*actor.PIDSet {
		subscribers := map[msg.MessageType]*actor.PIDSet{}
		for _, msgType := range msgTypes {
			subscribers[msgType] = actor.NewPIDSet()
		}
		return subscribers
	}

	return &Bot{
		peers:         actor.NewPIDSet(),
		loadedPlugins: make(map[plgn.Plugin]*PluginContract),
		remoter:       remoter,
		remotes:       treeset.NewWith(CmpRemote),
		activePlugins: treeset.NewWith(plgn.CmpPlugins),
		messageTypes:  configuration.SupportedMsgTypes,
		pluginLogger:  log.New(log.DefaultLevel, "[Plugin]"),
		subscribers:   initSubsribers(util.Values(&configuration.SupportedMsgTypes)),
		pluginRepoUrl: configuration.DefaultPluginRepoUrl,
	}
}
