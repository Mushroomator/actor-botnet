package bot

import (
	"net/url"
	"plugin"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/Mushroomator/actor-botnet/src/plgn"
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
	init()
	// ability to load a plugin
	loadPlugin(ident plgn.PluginIdentifier) (*plugin.Plugin, error)
	// load a plugin from the filesystem
	loadFsLocalPlugin(path string) (*plugin.Plugin, error)
	// load required functions and variables from plugin
	loadFunctionsAndVariablesFromPlugin(plgn plugin.Plugin) (*PluginContract, error)
}
