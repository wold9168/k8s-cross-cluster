package tailscaledclient

import (
	"context"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/types/key"
)

// Client wraps the local Tailscale client to provide a clean API
type Client struct {
	localClient *local.Client
}

// New creates a new Tailscale client
func New() *Client {
	return &Client{
		localClient: &local.Client{},
	}
}

// Status returns the current status of the Tailscale daemon
func (c *Client) Status(ctx context.Context) (*ipnstate.Status, error) {
	return c.localClient.Status(ctx)
}

// StatusWithoutPeers returns the current status of the Tailscale daemon without peer information
func (c *Client) StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error) {
	return c.localClient.StatusWithoutPeers(ctx)
}

// Self returns information about the current node
func (c *Client) Self(ctx context.Context) (*ipnstate.PeerStatus, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return nil, err
	}

	if status.Self == nil {
		return nil, nil
	}

	return status.Self, nil
}

// Peers returns a list of peer statuses
func (c *Client) Peers(ctx context.Context) ([]*ipnstate.PeerStatus, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return nil, err
	}

	peers := make([]*ipnstate.PeerStatus, 0)
	for _, peerID := range status.Peers() {
		peer := status.Peer[peerID]
		peers = append(peers, peer)
	}

	return peers, nil
}

// GetPeerByNodeID returns a specific peer by its node ID
func (c *Client) GetPeerByNodeID(ctx context.Context, nodeID key.NodePublic) (*ipnstate.PeerStatus, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return nil, err
	}

	peer, ok := status.Peer[nodeID]
	if !ok {
		return nil, nil
	}

	return peer, nil
}

// Up starts or configures the Tailscale daemon
func (c *Client) Up(ctx context.Context, opts ipn.Options) error {
	return c.localClient.Start(ctx, opts)
}

// Down stops the Tailscale daemon
func (c *Client) Down(ctx context.Context) error {
	prefs, err := c.localClient.GetPrefs(ctx)
	if err != nil {
		return err
	}
	newPrefs := prefs.Clone()
	newPrefs.WantRunning = false

	_, err = c.localClient.EditPrefs(ctx, &ipn.MaskedPrefs{
		Prefs: *newPrefs,
		WantRunningSet: true,
	})
	return err
}

// Logout disconnects the current node from the tailnet
func (c *Client) Logout(ctx context.Context) error {
	return c.localClient.Logout(ctx)
}
