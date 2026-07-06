# tunneler

Small TCP port tunneler for environments where SSH tunneling is not available.

Run `tunneler server` on a machine that can reach private/VPN-only hosts. Run `tunneler` on your local machine. The client listens on one or more localhost ports and forwards each TCP connection through the server to the configured remote host and port.

## Build

```sh
make build
```

Build binaries for Linux, macOS, and Windows under `dist/`:

```sh
make build-all
```

Useful development commands:

```sh
make tidy
make fmt
make lint
make test
make check
```

## Server

`server.json`:

```json
{
  "listenAddress": ":7000",
  "token": "change-me"
}
```

Run on the VPN/private-network machine:

```sh
./dist/tunneler server -config server.json
```

## Client

`client.json`:

```json
{
  "serverAddress": "vpn-host.example.com:7000",
  "token": "change-me",
  "tunnels": [
    {
      "name": "mssql",
      "targetHost": "10.0.0.15",
      "targetPort": 1433
    }
  ]
}
```

Run locally:

```sh
./dist/tunneler -config client.json
```

Then connect to `127.0.0.1:1433` from your local machine. Traffic is forwarded to `10.0.0.15:1433` from the server machine.

Multiple tunnels can be configured in the same client config by adding more entries to `tunnels`. `name` and `localPort` are optional. When `localPort` is omitted, the client uses the same port as `targetPort`.

Configured tunnels are lazy. The client only opens local listeners at startup. It connects to the tunnel server and target host only when something connects to the corresponding local port, so unused tunnels do not create remote connections.

## Notes

The token is a shared secret checked by the server before opening the target connection. It is not encryption. Use this only on trusted networks or place it behind a secure transport such as TLS/VPN/firewall rules.
