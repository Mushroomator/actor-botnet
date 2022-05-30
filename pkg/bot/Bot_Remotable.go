package bot

import (
	"fmt"

	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/configuration"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/msg"
	"github.com/Mushroomator/actor-bots-golang-plugins/pkg/util"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"github.com/emirpasic/gods/sets"
)

// Add remotes
func (state *Bot) AddRemote(host ...*Remote) {
	for _, remote := range host {
		state.remotes.Add(remote)
	}
}

// Remove remotes
func (state *Bot) RemoveRemote(host ...*Remote) {
	for _, remote := range host {
		state.remotes.Remove(remote)
	}
}

// Set all remotes
func (state *Bot) SetRemotes(remotes sets.Set) {
	state.remotes = remotes
}

// Get all remotes. Returns a copy of all the remotes.
func (state *Bot) Remotes() []*Remote {
	return util.CastArray(state.remotes.Values(), func(input any) *Remote {
		return input.(*Remote)
	})
}

// Handle request to spawn a new bot.
// Spawns a new bot at the given host
// Sends back a message to the sender (if known) with the PID of the spawned bot
func (state *Bot) handleSpawn(ctx actor.Context, message *msg.Spawn) {
	pid, err := state.SpawnBot(ctx, message.Host.Hostname, int(message.Host.Port))
	if err != nil {
		logger.Info("failed to spawn bot.", log.Error(err))
		return
	}
	if ctx.Sender() != nil {
		ctx.Send(ctx.Sender(), msg.NewSpawned(pid))
	}
}

// Handle message Spawned. Spawned notifies the receiver that a new bot was spawned and provides the PID of the newly created bot
// Add the newly created bot to peers
func (state *Bot) handleSpawned(ctx actor.Context, message *msg.Spawned) {
	state.AddPeer(message.Bot)
}

// Spawns a new bot on the given remote host
func (state *Bot) SpawnBot(ctx actor.Context, host string, port int) (*actor.PID, error) {

	// proto.actor does not properly deal with domains so make sure we have an IP address
	ip, err := util.GetIp(host)
	if err != nil {
		logger.Warn("failed to spawn bot", log.String("host", host), log.Error(err))
		return nil, err
	}

	// create a PID for a new peer
	address := fmt.Sprintf("%v:%v", ip.String(), port)
	spawnResponse, err := state.remoter.Spawn(address, "bot", configuration.RemoteSpawnTimeout)
	if err != nil {
		logger.Warn("failed to spawn bot", log.String("address", address), log.Error(err))
		return nil, err
	}

	// send new bot a created message providing it with the peers of this bot (exlcuding the newly created bot) and the remotes
	remotes := util.CastArray(state.remotes.Values(), func(input interface{}) *msg.RemoteAddress {
		remote := input.(*Remote)
		return msg.NewRemoteAddress(remote.Host, remote.Port)
	})

	// add remote location now
	state.AddRemote(&Remote{Host: host, Port: port})

	// send created message to new Bot supplying it with known remotes and peers
	ctx.Request(spawnResponse.GetPid(), msg.NewCreated(remotes, state.peers.Values()))
	state.AddPeer(spawnResponse.GetPid())
	return spawnResponse.GetPid(), nil
}

func KillBot(ctx actor.Context, pid *actor.PID) {
	ctx.Poison(pid)
}

// Handle *msg.Created message.
// Adds peers and remotes of creator to own peers and remotes.
func (state *Bot) handleCreated(ctx actor.Context, message *msg.Created) {
	remotes := util.CastArray(message.Remotes, func(input *msg.RemoteAddress) *Remote {
		return &Remote{
			Host: input.Hostname,
			Port: int(input.Port),
		}
	})
	state.AddPeer(message.Peers...)
	state.AddRemote(remotes...)
}

// Add a peer
func (state *Bot) AddPeer(pid ...*actor.PID) {
	for _, peer := range pid {
		if peer != nil {
			state.peers.Add(peer)
		}
	}
}

// Remove a peer
func (state *Bot) RemovePeer(pid ...*actor.PID) {
	for _, peer := range pid {
		if peer != nil {
			state.peers.Remove(peer)
		}
	}
}

// Set peers
func (state *Bot) SetPeers(peers *actor.PIDSet) {
	state.peers = peers
}

// Get peers. A copy of actual peers set is created.
func (state *Bot) Peers(pid *actor.PID) *actor.PIDSet {
	return state.peers.Clone()
}
