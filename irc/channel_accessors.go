package irc

// Accessor methods for the Channel struct to support peering

// GetName returns the channel name
func (c *Channel) GetName() string {
	c.RLock()
	defer c.RUnlock()
	return c.name
}

// GetTopic returns the channel topic
func (c *Channel) GetTopic() string {
	c.RLock()
	defer c.RUnlock()
	return c.topic
}

// SetTopic sets the channel topic
func (c *Channel) SetTopic(topic string) {
	c.Lock()
	defer c.Unlock()
	c.topic = topic
}

// GetModes returns the channel modes
func (c *Channel) GetModes() string {
	c.RLock()
	defer c.RUnlock()
	return c.modes
}

// SetModes sets the channel modes
func (c *Channel) SetModes(modes string) {
	c.Lock()
	defer c.Unlock()
	c.modes = modes
}

// AddClient adds a client to the channel
func (c *Channel) AddClient(client *Client) {
	c.Lock()
	defer c.Unlock()
	c.clients[client.GetNickname()] = client
}

// RemoveClient removes a client from the channel
func (c *Channel) RemoveClient(nickname string) {
	c.Lock()
	defer c.Unlock()
	delete(c.clients, nickname)
}

// HasClient checks if a client is in the channel
func (c *Channel) HasClient(nickname string) bool {
	c.RLock()
	defer c.RUnlock()
	_, exists := c.clients[nickname]
	return exists
}

// AddOperator adds a client as a channel operator
func (c *Channel) AddOperator(nickname string) {
	c.Lock()
	defer c.Unlock()
	c.operators[nickname] = true
}

// RemoveOperator removes a client as a channel operator
func (c *Channel) RemoveOperator(nickname string) {
	c.Lock()
	defer c.Unlock()
	delete(c.operators, nickname)
}

// IsOperator checks if a client is a channel operator
func (c *Channel) IsOperator(nickname string) bool {
	c.RLock()
	defer c.RUnlock()
	return c.operators[nickname]
}

// Broadcast sends a message to all clients in the channel
func (c *Channel) Broadcast(sender *Client, command string, params ...string) {
	c.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		clients = append(clients, client)
	}
	c.RUnlock()
	
	senderNick := ""
	if sender != nil {
		senderNick = sender.GetNickname()
	}
	
	for _, client := range clients {
		// Skip the sender if present
		if sender != nil && client.GetNickname() == senderNick {
			continue
		}
		
		client.sendMessage(command, params...)
	}
}
