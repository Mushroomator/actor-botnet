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
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"reflect"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/plgn"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
)

type SimpleBot struct {
	// list of neigboring bots
	peers *actor.PIDSet
	// list of loadedPlugins this bot has
	loadedPlugins map[plgn.PluginIdentifier]*PluginContract
	activePlugins *treeset.Set
	remotes       sets.Set
	messageTypes  map[string]msg.MessageType
	subscribers   map[msg.MessageType]*actor.PIDSet
	// plugin repository
	pluginRepoUrl *url.URL
}

// Create a new simple bot
func NewSimpleBot() *SimpleBot {
	initSubsribers := func(msgTypes []msg.MessageType) map[msg.MessageType]*actor.PIDSet {
		subscribers := map[msg.MessageType]*actor.PIDSet{}
		for _, msgType := range msgTypes {
			subscribers[msgType] = actor.NewPIDSet()
		}
		return subscribers
	}
	supportedMsgTypes := map[string]msg.MessageType{
		reflect.TypeOf(msg.Created{}).String():     msg.MessageType_CREATED,
		reflect.TypeOf(msg.Spawn{}).String():       msg.MessageType_SPAWN,
		reflect.TypeOf(msg.Spawned{}).String():     msg.MessageType_SPAWNED,
		reflect.TypeOf(msg.Run{}).String():         msg.MessageType_RUN,
		reflect.TypeOf(msg.Subscribe{}).String():   msg.MessageType_SUBSCRIBE,
		reflect.TypeOf(msg.Unsubscribe{}).String(): msg.MessageType_UNSUBSCRIBE,
		reflect.TypeOf(msg.LoadPlugin{}).String():  msg.MessageType_LOAD_PLUGIN,
		reflect.TypeOf(msg.Notify{}).String():      msg.MessageType_NOTIFY,
	}

	return &SimpleBot{
		peers:         actor.NewPIDSet(),
		loadedPlugins: make(map[plgn.PluginIdentifier]*PluginContract, initialNlSize),
		remotes:       treeset.NewWith(CmpRemote),
		activePlugins: treeset.NewWith(plgn.CmpPlugins),
		messageTypes:  supportedMsgTypes,
		subscribers:   initSubsribers(util.Values(&supportedMsgTypes)),
		pluginRepoUrl: defaultPluginRepoUrl,
	}
}

// Adds a new subscriber to for a certain messaget type
// If no message type is specified, a subscription for all messages is created
func (state *SimpleBot) AddSubscriber(subscriber *actor.PID, messageTypes ...msg.MessageType) {
	if subscriber != nil {
		if len(messageTypes) == 0 {
			messageTypes = util.Values(&state.messageTypes)
		}
		for _, msgTypes := range messageTypes {
			state.subscribers[msgTypes].Add(subscriber)
		}
	}
}

// Removes a new subscriber from a certain message type
// If no message type is specified, all subscriptions are removed
func (state *SimpleBot) RemoveSubscriber(unsubscriber *actor.PID, messageTypes ...msg.MessageType) {
	if unsubscriber != nil {
		if len(messageTypes) == 0 {
			messageTypes = util.Values(&state.messageTypes)
		}
		for _, msgTypes := range messageTypes {
			state.subscribers[msgTypes].Add(unsubscriber)
		}
	}
}

func (state *SimpleBot) notifySubscribers(ctx actor.Context, message interface{}) {
	msgType, ok := state.messageTypes[reflect.TypeOf(message).String()]
	// if not "ok", message type is unknown and we don't have any subscribers
	if ok {
		subscribers := state.subscribers[msgType]
		subscribers.ForEach(func(i int, pid *actor.PID) {
			ctx.Send(pid, msg.NewNotify(ctx.Self(), msgType))
		})
	}

}

func (state *SimpleBot) AddRemote(host ...*Remote) {
	for _, remote := range host {
		state.remotes.Add(remote)
	}
}

func (state *SimpleBot) RemoveRemote(host ...*Remote) {
	for _, remote := range host {
		state.remotes.Remove(remote)
	}
}

func (state *SimpleBot) SetRemotes(remotes sets.Set) {
	state.remotes = remotes
}

func (state *SimpleBot) Remotes() sets.Set {
	return state.remotes
}

func (state *SimpleBot) AddPeer(pid ...*actor.PID) {
	for _, peer := range pid {
		if peer != nil {
			state.peers.Add(peer)
		}
	}
}

func (state *SimpleBot) RemovePeer(pid ...*actor.PID) {
	for _, peer := range pid {
		if peer != nil {
			state.peers.Remove(peer)
		}
	}
}

func (state *SimpleBot) SetPeers(peers *actor.PIDSet) {
	state.peers = peers
}

func (state *SimpleBot) Peers(pid *actor.PID) *actor.PIDSet {
	return state.peers
}

func (state *SimpleBot) AddActivePlugin(plugin *plgn.PluginIdentifier) {
	state.activePlugins.Add(plugin)
}

func (state *SimpleBot) RemoveActivePlugin(plugin *plgn.PluginIdentifier) {
	state.activePlugins.Remove(plugin)
}

// Set URL to remote repository
func (state *SimpleBot) SetRemoteRepoUrl(url *url.URL) {
	state.pluginRepoUrl = url
}

// Get URL to remote repository
func (state *SimpleBot) RemoteRepoUrl() *url.URL {
	return state.pluginRepoUrl
}

// Handle *actor.Started message
func (state *SimpleBot) handleStarted(ctx actor.Context) {
	logger.Info("initializing bot...", log.PID("pid", ctx.Self()))
}

func (state *SimpleBot) loadPlugin(ident *plgn.PluginIdentifier) error {
	_, isInMem := state.loadedPlugins[*ident]
	if !isInMem {
		// plugin is NOT in memory already --> load it
		plgnFile, err := state.loadPluginFile(*ident)
		if err != nil {
			return fmt.Errorf("could not load plugin %v", ident.String())
		}
		loadedPlgn, err := state.loadFunctionsAndVariablesFromPlugin(plgnFile)
		if err != nil {
			return fmt.Errorf("could not load variables/ functions from loaded plugin %v", ident.String())
		}
		state.loadedPlugins[*ident] = loadedPlgn
	}
	return nil
}

// Handle *msg.LoadPlugin message
func (state *SimpleBot) handleLoadPlugin(ctx actor.Context, message *msg.LoadPlugin) {
	// check if plugin is already loaded
	pluginIdent := plgn.NewPluginIdentifier(message.Plugin.Name, message.Plugin.Version)
	err := state.loadPlugin(pluginIdent)
	if err != nil {
		logger.Warn("failed to load plugin", log.PID("pid", ctx.Self()), log.String("plugin", pluginIdent.String()), log.Error(err))
		return
	}
	logger.Info("plugin successfully loaded", log.PID("pid", ctx.Self()), log.String("plugin", pluginIdent.String()))
	// add plugin to the set of active plugins
	state.AddActivePlugin(pluginIdent)
	if message.RunAfterLoad {
		// send ourself a run message
		ctx.Send(ctx.Self(), msg.NewRun())
	}
}

// Handle *actor.Stopping message
func (state *SimpleBot) handleStopping(ctx actor.Context) {
	logger.Info("shutting down bot...", log.PID("pid", ctx.Self()))
	state.cleanup()
}

func (state *SimpleBot) cleanup() {
	err := os.RemoveAll(pathToPluginFiles)
	if err != nil {
		logger.Error("Failed to remove plugin files", log.Error(err), log.String("pluginDir", pathToPluginFiles))
	}
}

// Handle *actor.Stopped message
func (state *SimpleBot) handleStopped(ctx actor.Context) {
	logger.Info("bot shutdown.", log.PID("pid", ctx.Self()))
}

// Proto.Actor central Receive() method which gets passed all messages sent to the post box of this actor.
func (state *SimpleBot) Receive(ctx actor.Context) {
	// log type of message that was received
	message := ctx.Message()
	logger.Info("received message", log.PID("receiverActor", ctx.Self()), log.String("messageType", reflect.TypeOf(message).String()))
	// add the sender to the list of peers, it sender is specified and not the actor itself
	if sender := ctx.Sender(); sender != nil && sender != ctx.Self() {
		state.AddPeer(sender)
	}
	switch mssg := message.(type) {
	case *actor.Started:
		state.handleStarted(ctx)
	case *msg.Created:
		state.handleCreated(ctx, mssg)
	case *msg.Spawn:
		state.handleSpawn(ctx, mssg)
	case *msg.Spawned:
		state.handleSpawned(ctx, mssg)
	case *msg.LoadPlugin:
		state.handleLoadPlugin(ctx, mssg)
	case *msg.Subscribe:
		state.handleSubscribe(ctx, mssg)
	case *msg.Unsubscribe:
		state.handleUnsubscribe(ctx, mssg)
	case *msg.Run:
		state.handleRun(ctx)
	case *actor.Stopping:
		state.handleStopping(ctx)
	case *actor.Stopped:
		state.handleStopped(ctx)
	default:
		logger.Warn("Received unknown message")
		return
	}
	state.notifySubscribers(ctx, ctx.Message())
}

func (state *SimpleBot) handleSubscribe(ctx actor.Context, message *msg.Subscribe) {
	state.AddSubscriber(message.Subscriber, message.MessageTypes...)
}

func (state *SimpleBot) handleUnsubscribe(ctx actor.Context, message *msg.Unsubscribe) {
	state.RemoveSubscriber(message.Unsubscriber, message.MessageTypes...)
}

func (state *SimpleBot) handleCreated(ctx actor.Context, message *msg.Created) {
	remotes := util.CastArray(message.Remotes, func(input *msg.RemoteAddress) *Remote {
		return &Remote{
			Host: input.Hostname,
			Port: int(input.Port),
		}
	})
	state.AddPeer(message.Peers...)
	state.AddRemote(remotes...)
}

// handle msg.Run message
// Executes the Receive() message for every active plugin
func (state *SimpleBot) handleRun(ctx actor.Context) {
	toBeRemoved := make([]*plgn.PluginIdentifier, 0)
	// for each plugin execute the Receive method
	if state.activePlugins.Size() == 0 {
		logger.Info("Tried to invoke a plugin while no plugin was loaded")
	}
	state.activePlugins.Each(func(index int, value interface{}) {
		plugin := value.(*plgn.PluginIdentifier)
		if plgn, ok := state.loadedPlugins[*plugin]; ok {
			// call the plugins Receive() method asynchronously
			go plgn.Receive(state, ctx)
		} else {
			// should not happen, active plugins are automatically loaded plugins
			// should it happen (for whatever reason), handle the error gracefully and remove the plugin from the active plugins
			toBeRemoved = append(toBeRemoved, plugin)
			logger.Warn("Plugin is declared as active plugin but is not loaded in memory. Removed plugin from active plugins.", log.String("plugin", plugin.String()))
		}
	})
	// remove all "dangling" plugins
	if len(toBeRemoved) > 0 {
		for _, pluginTbr := range toBeRemoved {
			state.RemoveActivePlugin(pluginTbr)
		}
	}
}

// Load a plugin from a plugin file, i. e. a shared object (.so) file either from local file system or from remote repository if it is not found locally.
func (state *SimpleBot) loadPluginFile(ident plgn.PluginIdentifier) (*plugin.Plugin, error) {
	// try to load plugin from local filesystem first
	plgnPath, err := filepath.Abs(path.Join(pathToPluginFiles, ident.PluginName+"_"+ident.PluginVersion+".so"))
	if err != nil {
		return nil, err
	}

	lfsPlgn, lfsErr := state.loadFsLocalPlugin(plgnPath)
	if lfsErr == nil {
		return lfsPlgn, nil
	}
	logger.Info("plugin not found locally. Trying to download from remote repository.")
	// plugin could not be loaded from local file system (does not exist/ wrong permissions) in local filesystem
	// try downloading it from remote repo or peers
	remErr := state.downloadPlugin(ident, plgnPath)
	if remErr == nil {
		// plugin was successfully downloaded, now load it
		lfsPlgn, lfsErr := state.loadFsLocalPlugin(plgnPath)
		if lfsErr == nil {
			return lfsPlgn, nil
		}
	}

	// none of the above was successful!
	return nil, fmt.Errorf("plugin %v could not be found", ident.String())
}

// Download a plugin file, i. e. a shared object (.so) file from remote repository
func (state *SimpleBot) downloadPlugin(ident plgn.PluginIdentifier, dest string) error {
	// create URI for plugin
	urlPath, err := url.Parse(ident.PluginName + "_" + ident.PluginVersion + ".so")
	if err != nil {
		return err
	}
	urlStr := state.pluginRepoUrl.ResolveReference(urlPath).String()
	rc := make(chan util.HttpResponse)
	// try to download plugin
	logger.Info("Downloading plugin from remote repository", log.String("url", urlStr))
	go util.HttpGetAsync(urlStr, rc)
	// while request is pending open up destination file
	absDirPath, pathErr := filepath.Abs(pathToPluginFiles)
	if pathErr != nil {
		return pathErr
	}
	dirErr := os.MkdirAll(absDirPath, 0777)
	if dirErr != nil {
		logger.Info("could not create directories", log.String("path", dest), log.Error(dirErr))
		return dirErr
	}
	f, err := os.Create(dest)
	if err != nil {
		logger.Info("failed to open up file.", log.Error(err))
		return err
	}
	defer f.Close()
	resp := <-rc
	if resp.Err != nil {
		return resp.Err
	}
	if resp.Resp.StatusCode != 200 {
		return fmt.Errorf("could not download plugin from %v. status code: %v", urlPath, resp.Resp.StatusCode)
	}
	// file handle is acquired and request was successful: write request data to file
	defer resp.Resp.Body.Close()
	io.Copy(f, resp.Resp.Body)
	logger.Info("plugin successfully downloaded", log.String("url", urlStr), log.String("path", dest))
	return nil
}

// Load plugin, i. e. a shared object (.so) file from local filesystem.
func (state *SimpleBot) loadFsLocalPlugin(path string) (*plugin.Plugin, error) {
	pathRune := []rune(path)
	if len(pathRune) < 4 || string(pathRune[len(pathRune)-3:]) != ".so" {
		return nil, fmt.Errorf("invalid file extension %v for local plugin. File extension must be \".so\"", path)
	}
	logger.Info("loading plugin from local filesystem", log.String("path", path))
	plugin, err := plugin.Open(path)
	if err != nil {
		logger.Info("failed to load plugin locally", log.Error(err))
		return nil, err
	}

	return plugin, nil
}

// Load all required functions and variables from the plugin file, i. e. a shared object (.so) file.
func (state *SimpleBot) loadFunctionsAndVariablesFromPlugin(plgn *plugin.Plugin) (*PluginContract, error) {
	symbolName := "Receive"
	sym, err := plgn.Lookup(symbolName)
	if err != nil {
		return nil, err
	}
	receive, ok := sym.(func(bot *SimpleBot, ctx actor.Context))
	if !ok {
		return nil, fmt.Errorf("plugin is missing required method %v", symbolName)
	}

	pluginAttr := &PluginContract{
		Receive: receive,
	}
	return pluginAttr, nil
}

// Handle request to spawn a new bot.
// Spawns a new bot at the given host
// Sends back a message to the sender (if known) with the PID of the spawned bot
func (state *SimpleBot) handleSpawn(ctx actor.Context, message *msg.Spawn) {
	pid, err := state.spawnBot(ctx, message.Host.Hostname, int(message.Host.Port))
	if err != nil {
		logger.Info("failed to spawn bot.", log.Error(err))
		return
	}
	if ctx.Sender() != nil {
		ctx.Send(ctx.Sender(), msg.NewSpawned(pid))
	}
}

// Handle message Spawned. Spawned notifies the receiver that a new bot was spawned and provides the PID of the newly created bot
// Add the newly created bot to peers
func (state *SimpleBot) handleSpawned(ctx actor.Context, message *msg.Spawned) {
	state.AddPeer(message.Bot)
}

// Spawns a new bot on the given remote host
func (state *SimpleBot) spawnBot(ctx actor.Context, host string, port int) (*actor.PID, error) {
	// add remote location
	state.AddRemote(&Remote{Host: host, Port: port})
	// proto.actor does not properly deal with domains so make sure we have an IP address
	ip := net.ParseIP(host)
	if ip == nil {
		// ParseIP returns nil if the it is no valid IP
		// try to resolve an IP for the given domain
		var err error
		ip, err = util.ResolveHostnameToIp(host)
		if err != nil {
			logger.Warn("failed to resolve hostname", log.String("host", host), log.Error(err))
			return nil, fmt.Errorf("remote host %v could not be resolved. No bot spawned!", host)
		}
	}
	// create a PID for a new peer
	pid := actor.NewPID(fmt.Sprintf("%v:%v", ip.String(), port), "bot")
	// send new bot a created message providing it with the peers of this bot (exlcuding the newly created bot) and the remotes
	remotes := util.CastArray(state.remotes.Values(), func(input interface{}) *msg.RemoteAddress {
		remote := input.(*Remote)
		return msg.NewRemoteAddress(remote.Host, remote.Port)
	})
	ctx.Send(pid, msg.NewCreated(remotes, state.peers.Values()))
	state.AddPeer(pid)
	return pid, nil
}
