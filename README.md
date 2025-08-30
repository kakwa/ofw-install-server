# ofw-netinstall-server

Minimal RARP+TFTP server for netbooting old Open Firmware based computers:

* Sun Sparc64 (V240, Netra, T1000, etc)
* Apple PowerPC (PowerBook & PowerMac G3/G4/G5)
* IBM Power (P4, P5, etc)

Linux-only (uses AF_PACKET raw sockets).

It implements a rough (and vibe coded) rarp server + a TFTP server handling the "IP in Hexa" file path used by Open Firmware.

## Build

```bash
make
```

## Run

Requires `root` or `CAP_NET_RAW` capability on the binary.

```bash
# Configure an IP on the listening NIC
export BOOT_SERVER_IP=172.24.42.150
export BOOT_SERVER_NIC=enp0s25
sudo ip addr add ${BOOT_SERVER_IP}/24 dev ${BOOT_SERVER_NIC}

sudo ./ofw-netinstall-server -tftpdefault ofwboot.net -i ${BOOT_SERVER_NIC}
```

Options:

- `-i`: interface name to be used
- `-tftpdefault`: default file served in case we have the `IP in Hexa` file request typical of Open Firmware.


