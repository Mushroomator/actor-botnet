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
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
)

// Handle *actor.Started message
func (state *Bot) handleStarted(ctx actor.Context) {
	logger.Info("initializing bot...", log.PID("pid", ctx.Self()))
}

// Handle *actor.Stopping message
func (state *Bot) handleStopping(ctx actor.Context) {
	logger.Info("shutting down bot...", log.PID("pid", ctx.Self()))
	state.cleanupPlugins()
}

// Handle *actor.Stopped message
func (state *Bot) handleStopped(ctx actor.Context) {
	logger.Info("bot shutdown.", log.PID("pid", ctx.Self()))
}

// Handle *actor.Terminated message
func (state *Bot) handleTerminated(ctx actor.Context, message *actor.Terminated) {
	logger.Info("bot terminated.", log.PID("pid", ctx.Self()), log.PID("by", message.Who), log.String("reason", message.Why.String()))
}
