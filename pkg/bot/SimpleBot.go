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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"plugin"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/plgn"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/google/uuid"
)

type SimpleBot struct {
	// list of neigboring bots
	peers *actor.PIDSet
	// list of loadedPlugins this bot has
	loadedPlugins map[plgn.PluginIdentifier]*PluginContract
	activePlugins *treeset.Set
	remotes       sets.Set
	// plugin repository
	pluginRepoUrl *url.URL
}

// Create a new simple bot
func NewSimpleBot() *SimpleBot {
	return &SimpleBot{
		peers:         actor.NewPIDSet(),
		loadedPlugins: make(map[plgn.PluginIdentifier]*PluginContract, initialNlSize),
		remotes:       treeset.NewWithStringComparator(),
		activePlugins: treeset.NewWith(plgn.CmpPlugins),
		pluginRepoUrl: defaultPluginRepoUrl,
	}
}

func (state *SimpleBot) AddRemote(host string) {
	state.remotes.Add(host)
}

func (state *SimpleBot) RemoveRemote(host string) {
	state.remotes.Remove(host)
}

func (state *SimpleBot) SetRemotes(remotes sets.Set) {
	state.remotes = remotes
}

func (state *SimpleBot) Remotes() sets.Set {
	return state.remotes
}

func (state *SimpleBot) AddPeer(pid *actor.PID) {
	state.peers.Add(pid)
}

func (state *SimpleBot) RemovePeer(pid *actor.PID) {
	state.peers.Remove(pid)
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
	pluginIdent := plgn.NewPluginIdentifier(message.Name, message.Version)
	err := state.loadPlugin(pluginIdent)
	if err != nil {
		logger.Warn("failed to load plugin", log.PID("pid", ctx.Self()), log.String("plugin", pluginIdent.String()))
		return
	}
	logger.Info("plugin successfully loaded", log.PID("pid", ctx.Self()), log.String("plugin", pluginIdent.String()))
	// add plugin to the set of active plugins
	state.AddActivePlugin(pluginIdent)
}

// Handle *actor.Stopping message
func (state *SimpleBot) handleStopping(ctx actor.Context) {
	logger.Info("shutting down bot...", log.PID("pid", ctx.Self()))
}

// Handle *actor.Stopped message
func (state *SimpleBot) handleStopped(ctx actor.Context) {
	logger.Info("bot shutdown.", log.PID("pid", ctx.Self()))
}

// Proto.Actor central Receive() method which gets passed all messages sent to the post box of this actor.
func (state *SimpleBot) Receive(ctx actor.Context) {
	logger.Info("received message", log.PID("pid", ctx.Self()))
	switch mssg := ctx.Message().(type) {
	case *actor.Started:
		state.handleStarted(ctx)
	case msg.Created:
		state.handleCreated(ctx, &mssg)
	case msg.Spawn:
		state.handleSpawn(ctx, &mssg)
	case msg.Spawned:
		state.handleSpawned(ctx, &mssg)
	case msg.LoadPlugin:
		state.handleLoadPlugin(ctx, &mssg)
	case msg.Run:
		state.handleRun(ctx)
	case *actor.Stopping:
		state.handleStopping(ctx)
	case *actor.Stopped:
		state.handleStopped(ctx)
	default:
		logger.Warn("Received unknown message")
	}

}

func (state *SimpleBot) handleCreated(ctx actor.Context, message *msg.Created) {
	state.SetPeers(actor.NewPIDSet(message.Peers...))
	state.SetRemotes(treeset.NewWithStringComparator(message.Remotes))
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
	logger.Info("Plugin not found locally. Trying to download from remote repository.")
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
	pid, err := state.spawnBot(ctx, message.Host)
	if err != nil {
		logger.Info("failed to spawn bot.", log.Error(err))
		return
	}
	if ctx.Sender() != nil {
		ctx.Send(ctx.Sender(), msg.Spawned{Bot: pid})
	}
}

// Handle message Spawned. Spawned notifies the receiver that a new bot was spawned and provides the PID of the newly created bot
// Add the newly created bot to peers
func (state *SimpleBot) handleSpawned(ctx actor.Context, message *msg.Spawned) {
	state.AddPeer(message.Bot)
}

// Spawns a new bot on the given remote host
func (state *SimpleBot) spawnBot(ctx actor.Context, host string) (*actor.PID, error) {
	ip, err := util.ResolveHostnameToIp(host)
	if err != nil {
		return nil, fmt.Errorf("remote host %v could be resolved. No bot spawned!", host)
	}
	// add remote location
	state.AddRemote(ip.String())
	// create a PID for a new peer
	pid := actor.NewPID(host, uuid.NewString())
	// send new bot a created message providing it with the peers of this bot (exlcuding the newly created bot) and the remotes
	ctx.Send(pid, msg.Created{
		Remotes: util.CastInterfaceSliceToStringSlice(state.remotes.Values()),
		Peers:   state.peers.Values(),
	})
	state.AddPeer(pid)
	return pid, nil
}
