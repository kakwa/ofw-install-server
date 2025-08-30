## Manual Setup

For reference, if this project doesn't work for you, or want a more standard setup, here is the manual option (Debian/Ubuntu packages):

Install the necessary stuff:
```shell
apt install atftpd rarpd
```

Set Server NIC & IPs:
```shell
export BOOT_SERVER_NIC=enp0s25
export OFW_IP=172.24.42.51
export BOOT_SERVER_IP=172.24.42.150
```

Download & put the boot image in the correct TFTP location (IP Address in Hexa):
```shell
# Go in TFTP Directory
cd /srv/tftp/
# calculate the file name expected by Open Firmware (IP address in hexadecimal)
arg="`echo ${OFW_IP} | sed 's/\./ /g'`"
img_name=`printf "%.2X%.2X%.2X%.2X\n" $arg`

# Old Debian 6.0 boot image option
# Download Extract boot.img from old netboot Debian 6 package
wget https://archive.debian.org/debian/pool/main/d/debian-installer-netboot-images/debian-installer-6.0-netboot-sparc_20110106.squeeze4.b6_all.deb
mkdir debtmp && dpkg-deb -x *.deb debtmp/
find ./debtmp/ -name 'boot.img' -exec cp {} ./boot.img \;
rm -rf -- debian-installer-6.0-netboot-sparc_20110106.squeeze4.b6_all.deb ./debtmp/
# create hardlink to it
ln -f boot.img ${img_name}

# OpenBSD diskless option
# wget https://ftp.openbsd.org/pub/OpenBSD/7.7/sparc64/ofwboot.net
# ln -f ofwboot.net ${img_name}
```

Start the TFTP server:
```shell
systemctl start atftpd.service
```

Set the server IP:
```shell
ip addr add ${BOOT_SERVER_IP}/24 dev ${BOOT_SERVER_NIC}
```
Launch rarpd in the forground & start the Open Firmware Computer:
```
rarpd -e -dv ${BOOT_SERVER_NIC}
```

You should see log messages likes:
```shell
rarpd[16222]: RARP request from 00:03:ba:5b:ae:b3 on enp0s25
rarpd[16222]: not found in /etc/ethers
```

Grab the MAC address from the logs, and create the mapping:
```shell
export OFW_MAC="00:03:ba:5b:ae:b3"

# Normalize MAC (uppercase, no colons)
MAC_UPPER=$(echo "$OFW_MAC" | tr '[:lower:]' '[:upper:]')
MAC_NOPUNCT=$(echo "$MAC_UPPER" | tr -d ':')

# Hostname format: sparc-<MAC>
# note: could be any name, just avoid collisions
HOSTNAME="sparc-${MAC_NOPUNCT}"

# Make sure ethers file exists
touch /etc/ethers

# Add to /etc/ethers if not already present
grep -q -F "$MAC_UPPER $HOSTNAME" /etc/ethers || \
    echo "$MAC_UPPER $HOSTNAME" | sudo tee -a /etc/ethers

# Add to /etc/hosts if not already present
grep -q -F "$OFW_IP $HOSTNAME" /etc/hosts || \
    echo "$OFW_IP $HOSTNAME" | sudo tee -a /etc/hosts
```

On the next try, your Open Firmware computer should now get an IP, download the tftp file and boot from it.
