package irc

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	pb "github.com/presbrey/pkg/irc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

// connectToPeers connects to the peer servers specified in the configuration
func (s *Server) connectToPeers() error {
	for _, address := range s.config.PeerAddresses {
		conn, err := grpc.Dial(address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			log.Printf("Warning: failed to connect to peer %s: %v", address, err)
			continue
		}

		s.peers[address] = conn
		log.Printf("Connected to peer server at %s", address)

		// Initial state sync
		go s.syncWithPeer(address, conn)
	}

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

// relayMessageToPeers relays a message to all connected peer servers
func (s *Server) relayMessageToPeers(sender *Client, command string, params ...string) {
	req := &pb.MessageRequest{
		SenderServer: s.config.ServerName,
		OriginNick:   sender.nickname,
		OriginUser:   sender.username,
		OriginHost:   sender.hostname,
		Command:      command,
		Params:       params,
	}

	for address, conn := range s.peers {
		client := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := client.RelayMessage(ctx, req)
		if err != nil {
			log.Printf("Failed to relay message to peer %s: %v", address, err)
		}
	}
}

// notifyPeersClientJoined notifies all peers that a new client has joined
func (s *Server) notifyPeersClientJoined(client *Client) {
	req := &pb.ClientInfo{
		Nickname:   client.nickname,
		Username:   client.username,
		Hostname:   client.hostname,
		Realname:   client.realname,
		IsOperator: client.Modes.Operator,
		ServerName: s.config.ServerName,
	}

	for address, conn := range s.peers {
		peerClient := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := peerClient.ClientJoined(ctx, req)
		if err != nil {
			log.Printf("Failed to notify peer %s about new client: %v", address, err)
		}
	}
}

// notifyPeersClientLeft notifies all peers that a client has left
func (s *Server) notifyPeersClientLeft(client *Client) {
	req := &pb.ClientInfo{
		Nickname:   client.nickname,
		Username:   client.username,
		Hostname:   client.hostname,
		Realname:   client.realname,
		IsOperator: client.Modes.Operator,
		ServerName: s.config.ServerName,
	}

	for address, conn := range s.peers {
		peerClient := pb.NewIRCPeerClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := peerClient.ClientLeft(ctx, req)
		if err != nil {
			log.Printf("Failed to notify peer %s about client leaving: %v", address, err)
		}
	}
}

// RelayMessage implements the gRPC RelayMessage method
func (ps *peerServer) RelayMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	// Process the received message
	originIdent := fmt.Sprintf("%s!%s@%s", req.OriginNick, req.OriginUser, req.OriginHost)

	// Format the message
	var raw string
	if len(req.Params) > 0 {
		last := req.Params[len(req.Params)-1]
		if strings.Contains(last, " ") {
			params := append(req.Params[:len(req.Params)-1], ":"+last)
			raw = fmt.Sprintf(":%s %s %s", originIdent, req.Command, strings.Join(params, " "))
		} else {
			raw = fmt.Sprintf(":%s %s %s", originIdent, req.Command, strings.Join(req.Params, " "))
		}
	} else {
		raw = fmt.Sprintf(":%s %s", originIdent, req.Command)
	}

	// Determine the target to relay the message to
	var target string
	if len(req.Params) > 0 {
		target = req.Params[0]
	}

	// Relay the message to the appropriate target
	ps.server.RLock()
	defer ps.server.RUnlock()

	if target != "" && (target[0] == '#' || target[0] == '&') {
		// Channel message
		channel, exists := ps.server.channels[target]
		if exists {
			channel.RLock()
			for _, client := range channel.clients {
				client.sendRaw(raw)
			}
			channel.RUnlock()
		}
	} else if target != "" {
		// Private message to user
		client, exists := ps.server.clients[target]
		if exists {
			client.sendRaw(raw)
		}
	} else {
		// Broadcast to all clients
		for _, client := range ps.server.clients {
			client.sendRaw(raw)
		}
	}

	return &pb.MessageResponse{Success: true}, nil
}

// SyncState implements the gRPC SyncState method
func (ps *peerServer) SyncState(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	// Skip processing if the sender server is ourselves
	if req.SenderServer == ps.server.config.ServerName {
		return &pb.SyncResponse{Success: true}, nil
	}

	// Process received state
	ps.server.Lock()
	defer ps.server.Unlock()

	// Process channels
	for _, channelInfo := range req.Channels {
		channel, exists := ps.server.channels[channelInfo.Name]
		if !exists {
			// Create the channel
			channel = &Channel{
				name:      channelInfo.Name,
				topic:     channelInfo.Topic,
				modes:     channelInfo.Modes,
				clients:   make(map[string]*Client),
				operators: make(map[string]bool),
			}
			ps.server.channels[channelInfo.Name] = channel
		} else {
			// Update channel information
			channel.Lock()
			channel.topic = channelInfo.Topic
			channel.modes = channelInfo.Modes
			channel.Unlock()
		}
	}

	// Process clients
	for _, clientDetail := range req.Clients {
		// Check if the client already exists
		if _, exists := ps.server.clients[clientDetail.Nickname]; !exists {
			// Create a placeholder client
			client := &Client{
				nickname:   clientDetail.Nickname,
				username:   clientDetail.Username,
				hostname:   clientDetail.Hostname,
				realname:   clientDetail.Realname,
				server:     ps.server,
				channels:   make(map[string]bool),
				Modes:      UserMode{Operator: clientDetail.IsOperator},
				registered: true,
				lastPong:   time.Now(),
			}

			// Add the client to the server
			ps.server.clients[clientDetail.Nickname] = client

			// Add the client to their channels
			for _, channelName := range clientDetail.Channels {
				if channel, exists := ps.server.channels[channelName]; exists {
					channel.Lock()
					channel.clients[clientDetail.Nickname] = client
					channel.Unlock()

					client.Lock()
					client.channels[channelName] = true
					client.Unlock()
				}
			}
		}
	}

	return &pb.SyncResponse{Success: true}, nil
}

// ClientJoined implements the gRPC ClientJoined method
func (ps *peerServer) ClientJoined(ctx context.Context, req *pb.ClientInfo) (*pb.StatusResponse, error) {
	// Skip processing if the client is from our server
	if req.ServerName == ps.server.config.ServerName {
		return &pb.StatusResponse{Success: true}, nil
	}

	// Check if the client already exists
	ps.server.Lock()
	defer ps.server.Unlock()

	if _, exists := ps.server.clients[req.Nickname]; exists {
		return &pb.StatusResponse{
			Success:      false,
			ErrorMessage: "Client with that nickname already exists",
		}, nil
	}

	// Create a placeholder client
	client := &Client{
		nickname:   req.Nickname,
		username:   req.Username,
		hostname:   req.Hostname,
		realname:   req.Realname,
		server:     ps.server,
		channels:   make(map[string]bool),
		Modes:      UserMode{Operator: req.IsOperator},
		registered: true,
		lastPong:   time.Now(),
	}

	// Add the client to the server
	ps.server.clients[req.Nickname] = client

	return &pb.StatusResponse{Success: true}, nil
}

// ClientLeft implements the gRPC ClientLeft method
func (ps *peerServer) ClientLeft(ctx context.Context, req *pb.ClientInfo) (*pb.StatusResponse, error) {
	// Skip processing if the client is from our server
	if req.ServerName == ps.server.config.ServerName {
		return &pb.StatusResponse{Success: true}, nil
	}

	// Check if the client exists
	ps.server.Lock()
	defer ps.server.Unlock()

	client, exists := ps.server.clients[req.Nickname]
	if !exists {
		return &pb.StatusResponse{
			Success:      false,
			ErrorMessage: "Client not found",
		}, nil
	}

	// Remove the client from all channels
	for channelName := range client.channels {
		if channel, exists := ps.server.channels[channelName]; exists {
			channel.Lock()
			delete(channel.clients, client.nickname)
			delete(channel.operators, client.nickname)

			// If the channel is empty, remove it
			if len(channel.clients) == 0 {
				delete(ps.server.channels, channelName)
			}
			channel.Unlock()
		}
	}

	// Remove the client from the server
	delete(ps.server.clients, req.Nickname)

	return &pb.StatusResponse{Success: true}, nil
}
