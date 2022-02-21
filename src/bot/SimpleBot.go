package bot

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"plugin"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/Mushroomator/actor-botnet/src/plgn"
	"github.com/Mushroomator/actor-botnet/src/util"
)

type SimpleBot struct {
	// list of neigboring bots
	nl []*actor.PID
	// list of plugins this bot has
	plugins map[plgn.PluginIdentifier]interface{}
	// plugin repository
	pluginRepoUrl *url.URL
}

// Create a new simple bot
func NewSimpleBot() *SimpleBot {
	return &SimpleBot{
		nl:            make([]*actor.PID, initialNlSize),
		plugins:       make(map[plgn.PluginIdentifier]interface{}, initialNlSize),
		pluginRepoUrl: defaultPluginRepoUrl,
	}
}

func (state *SimpleBot) SetNl(nl []*actor.PID) {
	state.nl = nl
}

func (state *SimpleBot) Nl() []*actor.PID {
	return state.nl
}

func (state *SimpleBot) SetPlugins(plugins map[plgn.PluginIdentifier]interface{}) {
	state.plugins = plugins
}

func (state *SimpleBot) Plugins() map[plgn.PluginIdentifier]interface{} {
	return state.plugins
}

func (state *SimpleBot) SetRemoteRepoUrl(url *url.URL) {
	state.pluginRepoUrl = url
}

func (state *SimpleBot) RemoteRepoUrl() *url.URL {
	return state.pluginRepoUrl
}

func (state *SimpleBot) init(ctx actor.Context) {
	logger.Info("initializing bot...", log.PID("pid", ctx.Self()))
	pluginIdent := plgn.NewPluginIdentifier("Test", "1")
	plgn, err := state.loadPlugin(*pluginIdent)
	if err != nil {
		logger.Info("could not load plugin", log.Error(err), log.PID("pid", ctx.Self()))
		return
	}
	funcs, err := state.loadFunctionsAndVariablesFromPlugin(plgn)
	if err != nil {
		logger.Info("could not load variables/ functions from loaded plugin", log.Error(err), log.PID("pid", ctx.Self()))
		return
	}
	funcs.Receive(state, ctx)
}

// Proto.Actor central Receive() method which gets passed all messages sent to the post box of this actor.
func (state *SimpleBot) Receive(ctx actor.Context) {
	msg := ctx.Message()
	logger.Info("received message", log.PID("pid", ctx.Self()))
	switch msg.(type) {
	case *actor.Started:
		state.init(ctx)
	// case *msg.PluginReq:
	// 	res := state.providePlugin(msg)
	// case *msg.PluginRes:
	default:
		state.Receive(ctx)
	}

}

// Load a plugin.
func (state *SimpleBot) loadPlugin(ident plgn.PluginIdentifier) (*plugin.Plugin, error) {
	// try to load plugin from local filesystem first
	plgnPath := path.Join(pathToPluginFiles, ident.PluginName, "-"+ident.PluginVersion, ".so")

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

func (state *SimpleBot) downloadPlugin(ident plgn.PluginIdentifier, dest string) error {
	// create URI for plugin
	urlPath, err := url.Parse(ident.PluginName + "_" + ident.PluginVersion)
	if err != nil {
		return err
	}
	urlStr := state.pluginRepoUrl.ResolveReference(urlPath).String()
	rc := make(chan util.HttpResponse)
	// try to download plugin
	logger.Info("Downloading plugin from remote repository", log.String("url", urlStr))
	go util.HttpGetAsync(urlStr, rc)
	// while request is pending open up destination file
	f, err := os.Create(dest)
	if err != nil {
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
	logger.Info("Plugin successfully downloaded", log.String("url", urlStr), log.String("path", dest))
	return nil
}

// Load plugin from local filesystem.
func (state *SimpleBot) loadFsLocalPlugin(path string) (*plugin.Plugin, error) {
	pathRune := []rune(path)
	if len(pathRune) < 4 || string(pathRune[len(pathRune)-3:]) != ".so" {
		return nil, fmt.Errorf("invalid file extension %v for local plugin. File extesnion must be \".so\"", path)
	}
	logger.Info("Loading plugin from local filesystem", log.String("path", path))
	plugin, err := plugin.Open(path)
	if err != nil {
		logger.Debug("Failed to load plugin locally", log.Error(err))
		return nil, err
	}

	return plugin, nil
}

// Load all required functions and variables from the plugin.
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
