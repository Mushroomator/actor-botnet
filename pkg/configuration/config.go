package configuration

import (
	"net/url"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
)

const (
	RemoteSpawnTimeout = 1 * time.Hour
)

var (
	SupportedMsgTypes = map[string]msg.MessageType{
		// self-defined messages for a bot
		reflect.TypeOf(&msg.Created{}).String():      msg.MessageType_CREATED,
		reflect.TypeOf(&msg.Spawn{}).String():        msg.MessageType_SPAWN,
		reflect.TypeOf(&msg.Spawned{}).String():      msg.MessageType_SPAWNED,
		reflect.TypeOf(&msg.Subscribe{}).String():    msg.MessageType_SUBSCRIBE,
		reflect.TypeOf(&msg.Unsubscribe{}).String():  msg.MessageType_UNSUBSCRIBE,
		reflect.TypeOf(&msg.Notify{}).String():       msg.MessageType_NOTIFY,
		reflect.TypeOf(&msg.LoadPlugin{}).String():   msg.MessageType_LOAD_PLUGIN,
		reflect.TypeOf(&msg.UnloadPlugin{}).String(): msg.MessageType_UNLOAD_PLUGIN,
	}
	DefaultPluginRepoUrl = &url.URL{
		Scheme: "https",
		Host:   "go-plugin-repo.s3.eu-central-1.amazonaws.com",
	}
	PathToPluginFiles = determinePluginDir()
)

// Determines the directory where the plugins will be saved to
func determinePluginDir() string {
	pluginDir, ok := os.LookupEnv("PLUGIN_DIR")
	if ok {
		return pluginDir
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		return path.Join(homeDir, "plugins")
	}
}
