/*
Package tailscaledclient provides a client library for interacting with the local Tailscale daemon.

The library wraps the tailscale.com/client/local package with a more convenient interface,
allowing you to get status information, manage connections, and control the Tailscale daemon.

Example usage:

	client := tailscaledclient.New()
	ctx := context.TODO()

	status, err := client.Status(ctx)
	if err != nil {
		log.Fatal(err)
	}

	self, err := client.Self(ctx)
	if err != nil {
		log.Fatal(err)
	}

	peers, err := client.Peers(ctx)
	if err != nil {
		log.Fatal(err)
	}
*/
package tailscaledclient
