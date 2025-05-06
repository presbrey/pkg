package irc

import (
	pb "github.com/presbrey/pkg/irc/proto"
	"google.golang.org/grpc"
)

// The following methods are used by the peering module to interact with the Server

// AddPeer adds a peer connection to the server
func (s *Server) AddPeer(address string, conn *grpc.ClientConn) {
	s.Lock()
	defer s.Unlock()
	s.peers[address] = conn
}

// RemovePeer removes a peer connection
func (s *Server) RemovePeer(address string) {
	s.Lock()
	defer s.Unlock()
	delete(s.peers, address)
}

// ForEachPeer executes a function for each peer connection
func (s *Server) ForEachPeer(fn func(address string, conn *grpc.ClientConn)) {
	s.RLock()
	defer s.RUnlock()

	for address, conn := range s.peers {
		fn(address, conn)
	}
}

// GetChannel returns a channel by name or nil if not found
func (s *Server) GetChannel(name string) *Channel {
	s.RLock()
	defer s.RUnlock()
	return s.channels[name]
}

// CreateChannel creates a new channel with the given name
func (s *Server) CreateChannel(name string) *Channel {
	s.Lock()
	defer s.Unlock()

	if s.channels[name] != nil {
		return s.channels[name]
	}

	channel := &Channel{
		name:      name,
		clients:   make(map[string]*Client),
		operators: make(map[string]bool),
		voices:    make(map[string]bool),
		halfops:   make(map[string]bool),
		owners:    make(map[string]bool),
		admins:    make(map[string]bool),
		bans:      make(map[string]*BanEntry),
		modeArgs:  make(map[rune]string),
	}

	s.channels[name] = channel
	return channel
}

// GetClient returns a client by nickname or nil if not found
func (s *Server) GetClient(nickname string) *Client {
	s.RLock()
	defer s.RUnlock()
	return s.clients[nickname]
}

// RegisterRemoteClient registers a client from a peer server
func (s *Server) RegisterRemoteClient(client *Client) {
	s.Lock()
	defer s.Unlock()
	s.clients[client.nickname] = client
}

// RemoveClient removes a client from the server
func (s *Server) RemoveClient(nickname string) {
	s.Lock()
	defer s.Unlock()
	delete(s.clients, nickname)
}

// DispatchRemoteCommand handles a command from a remote server
func (s *Server) DispatchRemoteCommand(client *Client, command string, params []string) bool {
	// This would need to handle the command as if it were from the client
	// Simplified implementation for now
	switch command {
	case "PRIVMSG", "NOTICE":
		if len(params) < 2 {
			return false
		}

		target := params[0]
		message := params[1]

		// If target is a channel, relay to channel members
		if channel := s.GetChannel(target); channel != nil {
			channel.Broadcast(client, command, channel.name, message)
			return true
		}

		// If target is a user, relay to that user
		if targetClient := s.GetClient(target); targetClient != nil {
			targetClient.sendMessage(command, client.nickname, message)
			return true
		}

		return false

	// Add other command handlers as needed
	default:
		return false
	}
}

// BroadcastClientJoin announces a client join to all local clients
func (s *Server) BroadcastClientJoin(client *Client) {
	s.RLock()
	defer s.RUnlock()

	for _, localClient := range s.clients {
		if !localClient.RemoteOrigin {
			localClient.sendMessage("NICK", client.nickname)
		}
	}
}

// BroadcastClientQuit announces a client quit to all local clients
func (s *Server) BroadcastClientQuit(client *Client, reason string) {
	s.RLock()
	defer s.RUnlock()

	for _, localClient := range s.clients {
		if !localClient.RemoteOrigin {
			localClient.sendMessage("QUIT", reason)
		}
	}
}

// RemoveClientFromAllChannels removes a client from all channels
func (s *Server) RemoveClientFromAllChannels(client *Client, reason string) {
	s.RLock()
	channels := make([]*Channel, 0, len(s.channels))
	for _, channel := range s.channels {
		channels = append(channels, channel)
	}
	s.RUnlock()

	for _, channel := range channels {
		channel.Lock()
		if _, ok := channel.clients[client.nickname]; ok {
			delete(channel.clients, client.nickname)
			delete(channel.operators, client.nickname)
			delete(channel.voices, client.nickname)
			channel.Broadcast(client, "PART", channel.name, reason)
		}
		channel.Unlock()
	}
}

// BuildSyncRequest populates a sync request with channel and client information
func (s *Server) BuildSyncRequest(req *pb.SyncRequest) {
	s.RLock()
	defer s.RUnlock()

	// Add channels to the sync request
	for name, channel := range s.channels {
		channel.RLock()

		channelInfo := &pb.ChannelInfo{
			Name:      name,
			Topic:     channel.topic,
			Modes:     channel.modes,
			Clients:   make([]string, 0, len(channel.clients)),
			Operators: make([]string, 0, len(channel.operators)),
		}

		for nickname := range channel.clients {
			channelInfo.Clients = append(channelInfo.Clients, nickname)
		}

		for nickname := range channel.operators {
			channelInfo.Operators = append(channelInfo.Operators, nickname)
		}

		channel.RUnlock()
		req.Channels = append(req.Channels, channelInfo)
	}

	// Add clients to the sync request
	for _, client := range s.clients {
		// Skip remote clients in the sync to avoid loops
		if client.RemoteOrigin {
			continue
		}

		client.RLock()

		clientDetail := &pb.ClientDetail{
			Nickname:   client.nickname,
			Username:   client.username,
			Hostname:   client.hostname,
			Realname:   client.realname,
			IsOperator: client.Modes.Operator,
			Channels:   make([]string, 0, len(client.channels)),
		}

		for channelName := range client.channels {
			clientDetail.Channels = append(clientDetail.Channels, channelName)
		}

		client.RUnlock()
		req.Clients = append(req.Clients, clientDetail)
	}
}
