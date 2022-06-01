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
	"sync"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/configuration"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/plgn"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

// Get logger for use in plugins
func (state *Bot) PluginLogger() *log.Logger {
	return state.pluginLogger
}

// Get active plugins
func (state *Bot) ActivePlugins() []*plgn.PluginIdentifier {
	return util.CastArray(state.activePlugins.Values(), func(input any) *plgn.PluginIdentifier {
		return input.(*plgn.PluginIdentifier)
	})
}

// Add a plugin to active plugins
func (state *Bot) AddActivePlugin(plugin *plgn.PluginIdentifier) {
	state.activePlugins.Add(plugin)
}

// Remove a plugin from active plugins.
func (state *Bot) RemoveActivePlugin(plugin plgn.Plugin) {
	state.activePlugins.Remove(plugin)
}

// Handle *msg.Run message
// Execute the "Receive()" message for every active plugin
func (state *Bot) handleRun(ctx actor.Context) {
	toBeRemoved := make([]plgn.Plugin, 0)
	// for each plugin execute the Receive method
	if state.activePlugins.Size() == 0 {
		logger.Info("Tried to invoke a plugin while no plugin was loaded")
	}
	logger.Debug("Inovking plugins", log.Int("numberActivePlugins", state.activePlugins.Size()))
	// to have the context availabel in the plugins the bot must wait until all plugins have processed the messages
	// Use WaitGroup to wait until all plugins have finished. Plugins must indicate this using finished()
	var wg sync.WaitGroup
	wg.Add(state.activePlugins.Size())
	finished := func() {
		wg.Done()
	}
	state.activePlugins.Each(func(index int, value interface{}) {
		plugin := value.(plgn.Plugin)
		if plgn, ok := state.loadedPlugins[plugin]; ok {
			// call the plugins Receive() method concurrently

			logger.Debug("Executing plugin", log.String("pluginName", plugin.PluginName()), log.String("pluginVersion", plugin.PluginVersion()), log.PID("bot", ctx.Self()))
			go plgn.Receive(state, ctx, plugin, finished)
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

// Handle *msg.LoadPlugin message
func (state *Bot) handleLoadPlugin(ctx actor.Context, message *msg.LoadPlugin) {
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
	// if message.RunAfterLoad {
	// 	// send ourself a run message
	// 	ctx.Send(ctx.Self(), msg.NewRun())
	// }
}

// Handle *msg.UnloadPlugin message
func (state *Bot) handleUnloadPlugin(ctx actor.Context, message *msg.UnloadPlugin) {
	// check if plugin is already loaded
	pluginIdent := plgn.NewPluginIdentifier(message.Plugin.Name, message.Plugin.Version)
	state.RemoveActivePlugin(pluginIdent)
}

// Load a plugin
func (state *Bot) loadPlugin(ident plgn.Plugin) error {
	_, isInMem := state.loadedPlugins[ident]
	logger.Debug("plugin already in memory", log.String("pluginName", ident.PluginName()), log.String("pluginVersion", ident.PluginVersion()))
	if !isInMem {
		// plugin is NOT in memory already --> load it
		plgnFile, err := state.loadPluginFile(ident)
		if err != nil {
			return fmt.Errorf("could not load plugin %v. Reason: %v", ident.String(), err.Error())
		}
		loadedPlgn, err := state.loadFunctionsAndVariablesFromPlugin(plgnFile)
		if err != nil {
			return fmt.Errorf("could not load variables/ functions from loaded plugin %v. Reason: %v", ident.String(), err.Error())
		}
		state.loadedPlugins[ident] = loadedPlgn
	}
	return nil
}

// Load a plugin from a plugin file, i. e. a shared object (.so) file either from local file system or from remote repository if it is not found locally.
func (state *Bot) loadPluginFile(ident plgn.Plugin) (*plugin.Plugin, error) {
	// try to load plugin from local filesystem first
	plgnPath, err := filepath.Abs(path.Join(configuration.PathToPluginFiles, ident.PluginName()+"_"+ident.PluginVersion()+".so"))
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

// Load plugin from source
func (state *Bot) getPluginFromSource(ident plgn.Plugin, dest string) error {
	return state.downloadPlugin(ident, dest)
}

// Download a plugin file, i. e. a shared object (.so) file from remote repository
func (state *Bot) downloadPlugin(ident plgn.Plugin, dest string) error {
	// create URI for plugin
	urlPath, err := url.Parse(ident.PluginName() + "_" + ident.PluginVersion() + ".so")
	if err != nil {
		return err
	}
	urlStr := state.pluginRepoUrl.ResolveReference(urlPath).String()
	rc := make(chan util.HttpResponse)
	// try to download plugin
	logger.Info("Downloading plugin from remote repository", log.String("url", urlStr))
	go util.HttpGetAsync(urlStr, rc)
	// while request is pending open up destination file
	absDirPath, pathErr := filepath.Abs(configuration.PathToPluginFiles)
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
func (state *Bot) loadFsLocalPlugin(path string) (*plugin.Plugin, error) {
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
func (state *Bot) loadFunctionsAndVariablesFromPlugin(goPlugin *plugin.Plugin) (*PluginContract, error) {
	symbolName := "Receive"
	sym, err := goPlugin.Lookup(symbolName)
	if err != nil {
		return nil, err
	}
	receive, ok := sym.(func(bot *Bot, ctx actor.Context, plugin plgn.Plugin, finished FinishedFunc))
	if !ok {
		return nil, fmt.Errorf("plugin is missing required method %v", symbolName)
	}

	pluginAttr := &PluginContract{
		Receive: receive,
	}
	return pluginAttr, nil
}

// Cleanup plugin folder/ files
func (state *Bot) cleanupPlugins() {
	err := os.RemoveAll(configuration.PathToPluginFiles)
	if err != nil {
		logger.Error("Failed to remove plugin files", log.Error(err), log.String("pluginDir", configuration.PathToPluginFiles))
	}
}
