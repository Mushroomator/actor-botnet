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

	"github.com/Mushroomator/actor-bots/pkg/msg"
	"github.com/Mushroomator/actor-bots/pkg/plgn"
	"github.com/Mushroomator/actor-bots/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

type SimpleBot struct {
	// list of neigboring bots
	nl []*actor.PID
	// list of loadedPlugins this bot has
	loadedPlugins map[plgn.PluginIdentifier]*PluginContract
	activePlugin  *PluginContract
	// plugin repository
	pluginRepoUrl *url.URL
}

// Create a new simple bot
func NewSimpleBot() *SimpleBot {
	return &SimpleBot{
		nl:            make([]*actor.PID, initialNlSize),
		loadedPlugins: make(map[plgn.PluginIdentifier]*PluginContract, initialNlSize),
		activePlugin:  nil,
		pluginRepoUrl: defaultPluginRepoUrl,
	}
}

func (state *SimpleBot) SetNl(nl []*actor.PID) {
	state.nl = nl
}

func (state *SimpleBot) Nl() []*actor.PID {
	return state.nl
}

// Set plugin cache
func (state *SimpleBot) SetPlugins(plugins map[plgn.PluginIdentifier]*PluginContract) {
	state.loadedPlugins = plugins
}

// Get plugin cache
func (state *SimpleBot) Plugins() map[plgn.PluginIdentifier]*PluginContract {
	return state.loadedPlugins
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

// Handle *msg.LoadPlugin message
func (state *SimpleBot) handleLoadPlugin(ctx actor.Context, pluginIdent *plgn.PluginIdentifier) {
	// check if plugin is already loaded
	if plgn, isInMem := state.loadedPlugins[*pluginIdent]; isInMem {
		// plugin is in memory already --> make it the active plugin
		state.activePlugin = plgn
	} else {
		// plugin is not in memory --> load it
		state.loadPlugin(ctx, pluginIdent)
	}
}

// Load a plugin so new functionality is available
func (state *SimpleBot) loadPlugin(ctx actor.Context, pluginIdent *plgn.PluginIdentifier) {
	plgn, err := state.loadPluginFile(*pluginIdent)
	if err != nil {
		logger.Info("could not load plugin", log.Error(err), log.PID("pid", ctx.Self()))
		return
	}
	funcs, err := state.loadFunctionsAndVariablesFromPlugin(plgn)
	if err != nil {
		logger.Info("could not load variables/ functions from loaded plugin", log.Error(err), log.PID("pid", ctx.Self()))
		return
	}
	state.activePlugin = funcs
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
	case *msg.LoadPlugin:
		state.handleLoadPlugin(ctx, (*plgn.PluginIdentifier)(mssg))
	case *actor.Stopping:
		state.handleStopping(ctx)
	case *actor.Stopped:
		state.handleStopped(ctx)
	default:
		if state.activePlugin != nil {
			state.activePlugin.Receive(state, ctx)
		} else {
			logger.Info("Tried to invoke a plugin while no plugin was loaded")
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
		return nil, fmt.Errorf("invalid file extension %v for local plugin. File extesnion must be \".so\"", path)
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
