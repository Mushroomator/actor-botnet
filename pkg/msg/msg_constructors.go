package msg

import actor "github.com/asynkron/protoactor-go/actor"

func NewCreated(remotes []*RemoteAddress, peers []*actor.PID) *Created {
	return &Created{
		Remotes: remotes,
		Peers:   peers,
	}
}

func NewSpawn(host *RemoteAddress) *Spawn {
	return &Spawn{
		Host: host,
	}
}

func NewSpawned(bot *actor.PID) *Spawned {
	return &Spawned{
		Bot: bot,
	}
}

func NewLoadPlugin(plugin *PluginIdentifier) *LoadPlugin {
	return &LoadPlugin{
		Plugin: plugin,
	}
}

func NewUnloadPlugin(plugin *PluginIdentifier) *UnloadPlugin {
	return &UnloadPlugin{
		Plugin: plugin,
	}
}

func NewSubscribe(subscriber *actor.PID, messageTypes ...MessageType) *Subscribe {
	return &Subscribe{
		Subscriber:   subscriber,
		MessageTypes: messageTypes,
	}
}

func NewUnsubscribe(unsubscriber *actor.PID, messageTypes ...MessageType) *Unsubscribe {
	return &Unsubscribe{
		Unsubscriber: unsubscriber,
		MessageTypes: messageTypes,
	}
}

func NewNotify(source *actor.PID, messageType MessageType) *Notify {
	return &Notify{
		Source:      source,
		MessageType: messageType,
	}
}

/*
--------------
	Models
--------------
*/

func NewRemoteAddress(hostname string, port int) *RemoteAddress {
	return &RemoteAddress{
		Hostname: hostname,
		Port:     int32(port),
	}
}

func NewPluginIdentifier(name, version string) *PluginIdentifier {
	return &PluginIdentifier{
		Name:    name,
		Version: version,
	}
}
