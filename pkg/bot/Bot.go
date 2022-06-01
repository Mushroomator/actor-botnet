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
	"reflect"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

// Proto.Actor central Receive() method which gets passed all messages sent to the post box of this actor.
func (state *Bot) Receive(ctx actor.Context) {
	// log type of message that was received
	message := ctx.Message()
	logger.Debug("received message", log.PID("receiverActor", ctx.Self()), log.String("messageType", reflect.TypeOf(message).String()))
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
	case *msg.UnloadPlugin:
		state.handleUnloadPlugin(ctx, mssg)
	case *msg.Subscribe:
		state.handleSubscribe(ctx, mssg)
	case *msg.Unsubscribe:
		state.handleUnsubscribe(ctx, mssg)
	case *actor.Stopping:
		state.handleStopping(ctx)
	case *actor.Stopped:
		state.handleStopped(ctx)
	case *actor.Terminated:
		state.handleTerminated(ctx, mssg)
	case *msg.Notify:
		state.handleNotify(ctx, mssg)
	default:
		logger.Warn("Received unknown message")
		return
	}
	state.handleForwardMessageToPlugin(ctx)
	state.NotifySubscribers(ctx, ctx.Message())
}
