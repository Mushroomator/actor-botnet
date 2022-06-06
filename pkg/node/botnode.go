package node

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

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/bot"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/configuration"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type BotnetNode struct {
	system  *actor.ActorSystem
	remoter *remote.Remote
	host    string
	port    int
}

func createRemoter(system *actor.ActorSystem, host string, port int) *remote.Remote {
	options := remote.Configure(host, port)
	remoter := remote.NewRemote(system, options)
	// node should be capable of spawing an actor of kind bot
	remoter.Register("bot", actor.PropsFromProducer(func() actor.Actor {
		return bot.NewBot(remoter)
	}))
	return remoter
}

func NewBotnetNode(system *actor.ActorSystem, host string, port int) *BotnetNode {
	remoter := createRemoter(system, host, port)
	return &BotnetNode{
		system:  system,
		remoter: remoter,
		host:    host,
		port:    port,
	}
}

func (node *BotnetNode) Start() {
	node.remoter.Start()
}

func (node *BotnetNode) Shutdown(graceful bool) {
	node.remoter.Shutdown(graceful)
}

func (node *BotnetNode) SpawnBot(host string, port int) (*actor.PID, error) {
	ip, err := util.GetIp(host)
	if err != nil {
		return nil, err
	}
	address := fmt.Sprintf("%v:%v", ip.String(), port)
	resp, err := node.remoter.Spawn(address, "bot", configuration.RemoteSpawnTimeout)
	if err != nil {
		return nil, err
	}
	return resp.GetPid(), nil
}
