# sun-netinstall-server

Minimal RARP server for assigning IPv4 addresses to hosts based on their MAC address. Linux-only (uses AF_PACKET raw sockets).

## Build

```bash
make build
```

The binary will be placed in `bin/sun-netinstall-server`.

## Run (host)

Requires root or `CAP_NET_RAW` capability on the binary. Example:

```bash
sudo ./bin/sun-netinstall-server -i eth0 -map "52:54:00:12:34:56=192.168.1.10,aa:bb:cc:dd:ee:ff=192.168.1.11" -v
```

- `-i` interface name
- `-map` comma-separated `MAC=IPv4` pairs
- `-v` verbose logging (optional)

## Docker

Build image:

```bash
docker build -t sun-netinstall-server .
```

Run with host networking and raw socket capability:

```bash
docker run --rm \
  --network host \
  --cap-add NET_RAW \
  sun-netinstall-server \
  -i eth0 -map "52:54:00:12:34:56=192.168.1.10"
```

Note: interface name inside the container should match the host interface when using `--network host`.

## Development

- `make tidy` to sync dependencies
- `make fmt` to format code
- `make vet` to run static checks
