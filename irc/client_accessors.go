package irc

// Accessor methods for the Client struct to support peering

// GetNickname returns the client's nickname
func (c *Client) GetNickname() string {
	c.RLock()
	defer c.RUnlock()
	return c.nickname
}

// SetNickname sets the client's nickname
func (c *Client) SetNickname(nickname string) {
	c.Lock()
	defer c.Unlock()
	c.nickname = nickname
}

// GetUsername returns the client's username
func (c *Client) GetUsername() string {
	c.RLock()
	defer c.RUnlock()
	return c.username
}

// SetUsername sets the client's username
func (c *Client) SetUsername(username string) {
	c.Lock()
	defer c.Unlock()
	c.username = username
}

// GetRealname returns the client's real name
func (c *Client) GetRealname() string {
	c.RLock()
	defer c.RUnlock()
	return c.realname
}

// SetRealname sets the client's real name
func (c *Client) SetRealname(realname string) {
	c.Lock()
	defer c.Unlock()
	c.realname = realname
}

// GetHostname returns the client's hostname
func (c *Client) GetHostname() string {
	c.RLock()
	defer c.RUnlock()
	return c.hostname
}

// SetHostname sets the client's hostname
func (c *Client) SetHostname(hostname string) {
	c.Lock()
	defer c.Unlock()
	c.hostname = hostname
}

// IsOperator returns whether the client is an operator
func (c *Client) IsOperator() bool {
	c.RLock()
	defer c.RUnlock()
	return c.Modes.Operator
}

// SetOperator sets the client's operator status
func (c *Client) SetOperator(isOper bool) {
	c.Lock()
	defer c.Unlock()
	c.Modes.Operator = isOper
}

// IsRemote returns whether the client is from a remote server
func (c *Client) IsRemote() bool {
	c.RLock()
	defer c.RUnlock()
	return c.RemoteOrigin
}

// GetRemoteServer returns the name of the remote server this client is from
func (c *Client) GetRemoteServer() string {
	c.RLock()
	defer c.RUnlock()
	return c.RemoteServer
}

// SetRemoteServer sets the remote server name for this client
func (c *Client) SetRemoteServer(serverName string) {
	c.Lock()
	defer c.Unlock()
	c.RemoteServer = serverName
	c.RemoteOrigin = true
}
