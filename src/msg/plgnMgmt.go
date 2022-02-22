package msg

import "github.com/Mushroomator/actor-bots/src/plgn"

type SwitchActivePlugin plgn.PluginIdentifier

func NewSwitchActivePlugin(name string, version string) *SwitchActivePlugin {
	return (*SwitchActivePlugin)(plgn.NewPluginIdentifier(name, version))
}

type PluginReq plgn.PluginIdentifier

func NewPluginReq(name string, version string) *PluginReq {
	return (*PluginReq)(plgn.NewPluginIdentifier(name, version))
}

type PluginRes struct {
	PluginName string
	PluginFile []byte
}

func NewPluginRes(name string, version []byte) *PluginRes {
	return &PluginRes{
		PluginName: name,
		PluginFile: version,
	}
}
