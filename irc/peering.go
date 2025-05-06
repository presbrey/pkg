package irc

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/presbrey/pkg/irc/proto"
	"google.golang.org/grpc"
)

// Implementation of the gRPC server for peering
type peerServer struct {
	pb.UnimplementedIRCPeerServer
	server *Server
}

// startGRPCServer starts the gRPC server for peering
func (s *Server) startGRPCServer() error {
	// Create a gRPC server
	s.peerServer = grpc.NewServer()

	// Register our service
	pb.RegisterIRCPeerServer(s.peerServer, &peerServer{server: s})

	// Start listening
	lis, err := net.Listen("tcp", s.config.GRPCBindAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.GRPCBindAddr, err)
	}

	// Start serving in a goroutine
	go func() {
		if err := s.peerServer.Serve(lis); err != nil {
			log.Printf("Failed to serve gRPC: %v", err)
		}
	}()

	return nil
}

// syncWithPeer synchronizes state with a peer server
func (s *Server) syncWithPeer(address string, conn *grpc.ClientConn) {
	client := pb.NewIRCPeerClient(conn)

	// Build the sync request
	req := &pb.SyncRequest{
		SenderServer: s.config.ServerName,
		Channels:     make([]*pb.ChannelInfo, 0),
		Clients:      make([]*pb.ClientDetail, 0),
	}

	// Add channels to the sync request
	s.RLock()
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
	s.RUnlock()

	// Send the sync request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.SyncState(ctx, req)
	if err != nil {
		log.Printf("Failed to sync with peer %s: %v", address, err)
		return
	}

	if !resp.Success {
		log.Printf("Peer %s rejected sync: %s", address, resp.ErrorMessage)
	} else {
		log.Printf("Successfully synced with peer %s", address)
	}
}
