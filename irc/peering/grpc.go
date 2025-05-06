package peering

import (
	"context"
	"fmt"
	"log"

	"github.com/presbrey/pkg/irc"
	pb "github.com/presbrey/pkg/irc/proto"
)

// RelayMessage implements the gRPC RelayMessage method
func (s *PeerServer) RelayMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	log.Printf("Received message relay from %s: %s %v", req.SenderServer, req.Command, req.Params)

	// Check if this is a message we should handle
	if req.SenderServer == s.manager.server.GetName() {
		// This is our own message that was relayed back, ignore it
		return &pb.MessageResponse{Success: true}, nil
	}

	// Create a "virtual" client representation for the remote user
	virtualClient := &irc.Client{
		RemoteOrigin: true,
		RemoteServer: req.SenderServer,
	}
	
	// Set client properties from the request
	virtualClient.SetNickname(req.OriginNick)
	virtualClient.SetUsername(req.OriginUser)
	virtualClient.SetHostname(req.OriginHost)

	// Dispatch the command
	success := s.manager.server.DispatchRemoteCommand(virtualClient, req.Command, req.Params)

	return &pb.MessageResponse{Success: success}, nil
}

// SyncState implements the gRPC SyncState method
func (s *PeerServer) SyncState(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	log.Printf("Received sync request from %s with %d channels and %d clients", 
		req.SenderServer, len(req.Channels), len(req.Clients))

	// Process channels from the sync request
	for _, channelInfo := range req.Channels {
		// Check if the channel already exists
		channel := s.manager.server.GetChannel(channelInfo.Name)
		if channel == nil {
			// Create the channel
			channel = s.manager.server.CreateChannel(channelInfo.Name)
			channel.SetTopic(channelInfo.Topic)
			channel.SetModes(channelInfo.Modes)
		}
	}

	// Process clients from the sync request
	for _, clientDetail := range req.Clients {
		// Check if the client is already registered locally
		existingClient := s.manager.server.GetClient(clientDetail.Nickname)
		if existingClient != nil {
			// Skip clients we already know about
			continue
		}

		// Create a virtual client for the remote user
		virtualClient := &irc.Client{
			RemoteOrigin: true,
			RemoteServer: req.SenderServer,
		}
		
		// Set client properties
		virtualClient.SetNickname(clientDetail.Nickname)
		virtualClient.SetUsername(clientDetail.Username)
		virtualClient.SetHostname(clientDetail.Hostname)
		virtualClient.SetRealname(clientDetail.Realname)
		
		if clientDetail.IsOperator {
			virtualClient.SetOperator(true)
		}

		// Register the virtual client
		s.manager.server.RegisterRemoteClient(virtualClient)

		// Add the client to channels
		for _, channelName := range clientDetail.Channels {
			channel := s.manager.server.GetChannel(channelName)
			if channel != nil {
				channel.AddClient(virtualClient)
				
				// Check if client should be an operator in this channel
				for _, op := range req.Channels {
					if op.Name == channelName {
						for _, opNick := range op.Operators {
							if opNick == clientDetail.Nickname {
								channel.AddOperator(clientDetail.Nickname)
								break
							}
						}
					}
				}
			}
		}
	}

	return &pb.SyncResponse{Success: true}, nil
}

// ClientJoined implements the gRPC ClientJoined method
func (s *PeerServer) ClientJoined(ctx context.Context, req *pb.ClientInfo) (*pb.StatusResponse, error) {
	log.Printf("Received client joined notification: %s@%s from %s", 
		req.Nickname, req.Hostname, req.ServerName)

	// Check if we already know about this client
	existingClient := s.manager.server.GetClient(req.Nickname)
	if existingClient != nil {
		// Client already exists, can happen during network splits, etc.
		return &pb.StatusResponse{Success: true}, nil
	}

	// Create a virtual client for the remote user
	virtualClient := &irc.Client{
		RemoteOrigin: true,
		RemoteServer: req.ServerName,
	}
	
	// Set client properties
	virtualClient.SetNickname(req.Nickname)
	virtualClient.SetUsername(req.Username)
	virtualClient.SetHostname(req.Hostname)
	virtualClient.SetRealname(req.Realname)
	
	if req.IsOperator {
		virtualClient.SetOperator(true)
	}

	// Register the virtual client
	s.manager.server.RegisterRemoteClient(virtualClient)

	// Notify all local clients about the new user
	s.manager.server.BroadcastClientJoin(virtualClient)

	return &pb.StatusResponse{Success: true}, nil
}

// ClientLeft implements the gRPC ClientLeft method
func (s *PeerServer) ClientLeft(ctx context.Context, req *pb.ClientInfo) (*pb.StatusResponse, error) {
	log.Printf("Received client left notification: %s from %s", 
		req.Nickname, req.ServerName)

	// Find the client
	client := s.manager.server.GetClient(req.Nickname)
	if client == nil {
		// Client not found, possibly already removed
		return &pb.StatusResponse{Success: true}, nil
	}

	// Verify this is a remote client from the correct server
	if !client.IsRemote() || client.GetRemoteServer() != req.ServerName {
		return &pb.StatusResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Client %s is not from server %s", req.Nickname, req.ServerName),
		}, nil
	}

	// Remove the client from all channels
	s.manager.server.RemoveClientFromAllChannels(client, "Left remote server")

	// Remove the client from the server
	s.manager.server.RemoveClient(client.GetNickname())

	// Notify all local clients about the user leaving
	s.manager.server.BroadcastClientQuit(client, "Left remote server")

	return &pb.StatusResponse{Success: true}, nil
}
