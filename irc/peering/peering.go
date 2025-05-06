package peering

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/presbrey/pkg/irc"
	pb "github.com/presbrey/pkg/irc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// StartGRPCServer starts the gRPC server for peering
func (m *Manager) StartGRPCServer(bindAddr string) error {
	// Create a gRPC server
	m.peerServer = grpc.NewServer()

	// Register our service
	pb.RegisterIRCPeerServer(m.peerServer, &PeerServer{manager: m})

	// Start listening
	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bindAddr, err)
	}

	// Start serving in a goroutine
	go func() {
		if err := m.peerServer.Serve(lis); err != nil {
			log.Printf("Failed to serve gRPC: %v", err)
		}
	}()

	log.Printf("GRPC Peer Server started on %s", bindAddr)
	return nil
}

// StopGRPCServer stops the gRPC server
func (m *Manager) StopGRPCServer() {
	if m.peerServer != nil {
		m.peerServer.GracefulStop()
		log.Println("GRPC Peer Server stopped")
	}
}

// ConnectToPeers connects to the peer servers specified in the addresses list
func (m *Manager) ConnectToPeers(addresses []string) error {
	for _, address := range addresses {
		conn, err := grpc.Dial(address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			log.Printf("Warning: failed to connect to peer %s: %v", address, err)
			continue
		}

		// Store the connection (we assume Server.peers is accessible)
		m.server.AddPeer(address, conn)
		log.Printf("Connected to peer server at %s", address)

		// Initial state sync
		go m.syncWithPeer(address, conn)
	}

	return nil
}

// syncWithPeer synchronizes state with a peer server
func (m *Manager) syncWithPeer(address string, conn *grpc.ClientConn) {
	// Create and populate the sync context
	ctx := SyncEventContext{
		Server:      m.server,
		PeerAddress: address,
		PeerConn:    conn,
	}

	// Run hooks for sync events
	m.syncEventHooks.RunAll(ctx)

	// Create the client for the peer
	client := pb.NewIRCPeerClient(conn)

	// Build the sync request
	req := &pb.SyncRequest{
		SenderServer: m.server.GetName(),
		Channels:     make([]*pb.ChannelInfo, 0),
		Clients:      make([]*pb.ClientDetail, 0),
	}

	// Add channels and clients to the sync request
	m.server.BuildSyncRequest(req)

	// Send the sync request
	rpcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.SyncState(rpcCtx, req)
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

// Register registers all necessary hooks for peering functionality
func (m *Manager) Register() {
	// Register hook for client joins
	m.clientJoinHooks.Register(func(ctx ClientEventContext) error {
		m.notifyPeersClientJoined(ctx.Client)
		return nil
	})

	// Register hook for client leaves
	m.clientLeaveHooks.Register(func(ctx ClientEventContext) error {
		m.notifyPeersClientLeft(ctx.Client)
		return nil
	})

	// Register hook for message relay
	m.messageRelayHooks.Register(func(ctx MessageEventContext) error {
		m.relayMessageToPeers(ctx.Sender, ctx.Command, ctx.Params...)
		return nil
	})
}

// notifyPeersClientJoined notifies all peers that a new client has joined
func (m *Manager) notifyPeersClientJoined(client *irc.Client) {
	req := &pb.ClientInfo{
		Nickname:   client.GetNickname(),
		Username:   client.GetUsername(),
		Hostname:   client.GetHostname(),
		Realname:   client.GetRealname(),
		IsOperator: client.IsOperator(),
		ServerName: m.server.GetName(),
	}

	m.server.ForEachPeer(func(address string, conn *grpc.ClientConn) {
		peerClient := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := peerClient.ClientJoined(ctx, req)
		if err != nil {
			log.Printf("Failed to notify peer %s about new client: %v", address, err)
		}
	})
}

// notifyPeersClientLeft notifies all peers that a client has left
func (m *Manager) notifyPeersClientLeft(client *irc.Client) {
	req := &pb.ClientInfo{
		Nickname:   client.GetNickname(),
		Username:   client.GetUsername(),
		Hostname:   client.GetHostname(),
		Realname:   client.GetRealname(),
		IsOperator: client.IsOperator(),
		ServerName: m.server.GetName(),
	}

	m.server.ForEachPeer(func(address string, conn *grpc.ClientConn) {
		peerClient := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := peerClient.ClientLeft(ctx, req)
		if err != nil {
			log.Printf("Failed to notify peer %s about client leaving: %v", address, err)
		}
	})
}

// relayMessageToPeers relays a message to all connected peer servers
func (m *Manager) relayMessageToPeers(sender *irc.Client, command string, params ...string) {
	req := &pb.MessageRequest{
		SenderServer: m.server.GetName(),
		OriginNick:   sender.GetNickname(),
		OriginUser:   sender.GetUsername(),
		OriginHost:   sender.GetHostname(),
		Command:      command,
		Params:       params,
	}

	m.server.ForEachPeer(func(address string, conn *grpc.ClientConn) {
		client := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := client.RelayMessage(ctx, req)
		if err != nil {
			log.Printf("Failed to relay message to peer %s: %v", address, err)
		}
	})
}
