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
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
)

// Handle *msg.Subsribe message
func (state *Bot) handleSubscribe(ctx actor.Context, message *msg.Subscribe) {
	state.AddSubscriber(message.Subscriber, message.MessageTypes...)
}

// Handle *msg.Unsubscribe message
func (state *Bot) handleUnsubscribe(ctx actor.Context, message *msg.Unsubscribe) {
	state.RemoveSubscriber(message.Unsubscriber, message.MessageTypes...)
}

// Handle *msg.Notify message
func (state *Bot) handleNotify(ctx actor.Context, message *msg.Notify) {

}

// Get subscribers
func (state *Bot) Subscribers() map[msg.MessageType]*actor.PIDSet {
	return util.CopyMap(state.subscribers)
}

// Adds a new subscriber to for a certain messaget type
// If no message type is specified, a subscription for all messages is created
func (state *Bot) AddSubscriber(subscriber *actor.PID, messageTypes ...msg.MessageType) {
	if subscriber != nil {
		if len(messageTypes) == 0 {
			messageTypes = util.Values(&state.messageTypes)
		}
		for _, msgTypes := range messageTypes {
			state.subscribers[msgTypes].Add(subscriber)
		}
	}
}

// Removes a new subscriber from a certain message type
// If no message type is specified, all subscriptions are removed
func (state *Bot) RemoveSubscriber(unsubscriber *actor.PID, messageTypes ...msg.MessageType) {
	if unsubscriber != nil {
		if len(messageTypes) == 0 {
			messageTypes = util.Values(&state.messageTypes)
		}
		for _, msgTypes := range messageTypes {
			state.subscribers[msgTypes].Add(unsubscriber)
		}
	}
}

// Notify relevant subscribers that an event they have subscribed to has occurred.
func (state *Bot) NotifySubscribers(ctx actor.Context, message interface{}) {
	msgType, ok := state.messageTypes[reflect.TypeOf(message).String()]
	// if not "ok", message type is unknown and we don't have any subscribers
	if ok {
		subscribers := state.subscribers[msgType]
		subscribers.ForEach(func(i int, pid *actor.PID) {
			ctx.Send(pid, msg.NewNotify(ctx.Self(), msgType))
		})
	}

}
