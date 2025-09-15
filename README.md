# ofw-install-server

[![CI](https://github.com/kakwa/ofw-install-server/actions/workflows/ci.yml/badge.svg)](https://github.com/kakwa/ofw-install-server/actions/workflows/ci.yml)

## Presentation

Minimal RARP + TFTP + optional BOOTP, NFSv2, and HTTP servers for netbooting Sun Open Firmware systems (V240, Netra X1/V100, T1000, etc.).

Linux-only (uses `AF_PACKET` raw sockets).

It implements a minimal RARP server and a TFTP server that also handles the "IP in Hexa" filename pattern used by Open Firmware.

Optional BOOTP, NFSv2 and HTTP helpers are included to also simplify end-to-end OpenBSD & NetBSD installs.

It's meant to be a simpler option than the [traditional setup](https://github.com/kakwa/ofw-install-server/blob/main/MANUAL_SETUP.md).

## Build

```bash
make
```

## Repository layout

- `nfs/`: Minimal NFSv2, mountd, and portmap (RPC) server
- `rarp/`: RARP server
- `bootp/`: BOOTP/DHCP server
- `tftp/`: TFTP server
- `http/`: Tiny HTTP file server
- `utils/`: IP allocation and small utilities
- `main.go`: CLI entrypoint

## Run

Requires `root` or `CAP_NET_RAW` capability on the binary.

If necessary, prepare the NIC on your host and set variables used below:

```bash
export BOOT_SERVER_IP=172.24.42.150
export BOOT_SERVER_NIC=enp0s25
sudo ip addr add ${BOOT_SERVER_IP}/24 dev ${BOOT_SERVER_NIC}
```

### OpenBSD Install

Prepare files:

```shell
# Tweak it to the latest versions
export OPENBSD_VERSION="7.7"
export NETBSD_VERSION="10.1"

# If you want to try your luck with the OpenBSD ofwboot.net
# wget "https://ftp.openbsd.org/pub/OpenBSD/${OPENBSD_VERSION}/sparc64/ofwboot.net"

# Recover bootloader from NetBSD
wget https://cdn.netbsd.org/pub/NetBSD/NetBSD-${NETBSD_VERSION}/sparc64/installation/netboot/ofwboot.net
# Recover OpenBSD install RamDisk
wget "https://ftp.openbsd.org/pub/OpenBSD/${OPENBSD_VERSION}/sparc64/bsd.rd"
# Recover an autoinstall file
wget https://raw.githubusercontent.com/kakwa/silly-sun-server/refs/heads/main/misc/openbsd-autoinstall.conf
```

Start the install server:

```shell
sudo ./ofw-install-server -iface ${BOOT_SERVER_NIC} -rarp \
  -tftp -tftp-file ./ofwboot.net \
  -bootp \
  -nfs -nfs-file ./bsd.rd \
  -http -http-file ./openbsd-autoinstall.conf
```

### NetBSD Install

Prepare files:

```shell
# Tweak it to the latest version
export NETBSD_VERSION="10.1"

wget "https://cdn.netbsd.org/pub/NetBSD/NetBSD-${NETBSD_VERSION}/sparc64/installation/netboot/ofwboot.net"
wget "https://cdn.netbsd.org/pub/NetBSD/NetBSD-${NETBSD_VERSION}/sparc64/binary/kernel/netbsd-INSTALL.gz"
gunzip netbsd-INSTALL.gz
```

Start the server:

```shell
sudo ./ofw-install-server -iface ${BOOT_SERVER_NIC} -rarp \
  -tftp -tftp-file ./ofwboot.net \
  -bootp \
  -nfs -nfs-file ./netbsd-INSTALL
```

### Flags

- `-iface`: interface to bind (default: `enp0s25`)
- `-rarp`: enable built-in RARP server
- `-tftp`: enable built-in TFTP server
- `-tftp-file`: file to serve via TFTP (used for ofwboot.net)
- `-bootp`: enable BOOTP/DHCP helper
- `-bootp-rootpath`: BOOTP root-path option
- `-bootp-filename`: BOOTP bootfile/filename option
- `-nfs`: enable minimal NFSv2 server
- `-nfs-file`: file served over NFSv2 reads (INSTALL ramdisk or bsd.rd)
- `-http`: enable tiny HTTP server
- `-http-file`: file served by HTTP for all requests (e.g., autoinstall config)
