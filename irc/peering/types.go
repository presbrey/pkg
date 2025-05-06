// Package peering provides IRC server peering functionality using hooks
package peering

import (
	"github.com/presbrey/pkg/hooks"
	"github.com/presbrey/pkg/irc"
	pb "github.com/presbrey/pkg/irc/proto"
	"google.golang.org/grpc"
)

// ClientEventContext is used for client join/leave events
type ClientEventContext struct {
	Server *irc.Server
	Client *irc.Client
}

// MessageEventContext is used for message relay events
type MessageEventContext struct {
	Server   *irc.Server
	Sender   *irc.Client
	Command  string
	Params   []string
}

// ChannelEventContext is used for channel-related events
type ChannelEventContext struct {
	Server  *irc.Server
	Channel *irc.Channel
	Client  *irc.Client // Optional, may be nil for some events
}

// SyncEventContext is used for state synchronization
type SyncEventContext struct {
	Server       *irc.Server
	PeerAddress  string
	PeerConn     *grpc.ClientConn
}

// Manager handles IRC peering functionality using hooks
type Manager struct {
	server             *irc.Server
	peerServer         *grpc.Server
	clientJoinHooks    *hooks.Registry[ClientEventContext]
	clientLeaveHooks   *hooks.Registry[ClientEventContext]
	messageRelayHooks  *hooks.Registry[MessageEventContext]
	channelEventHooks  *hooks.Registry[ChannelEventContext]
	syncEventHooks     *hooks.Registry[SyncEventContext]
}

// NewManager creates a new peering manager
func NewManager(server *irc.Server) *Manager {
	return &Manager{
		server:             server,
		clientJoinHooks:    hooks.NewRegistry[ClientEventContext](),
		clientLeaveHooks:   hooks.NewRegistry[ClientEventContext](),
		messageRelayHooks:  hooks.NewRegistry[MessageEventContext](),
		channelEventHooks:  hooks.NewRegistry[ChannelEventContext](),
		syncEventHooks:     hooks.NewRegistry[SyncEventContext](),
	}
}

// PeerServer is the gRPC server implementation for peering
type PeerServer struct {
	pb.UnimplementedIRCPeerServer
	manager *Manager
}
