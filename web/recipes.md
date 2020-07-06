# Recipies

Here are some examples of things you might be able to use nsbox for.

## Running Docker inside an nsbox container

::: warning
If you're on Fedora 31+, note that [cgroups v2 is enabled by default](
https://www.redhat.com/sysadmin/fedora-31-control-group-v2), which breaks Docker.
In order to use cgroups v1 instead, you can add the `systemd.unified_cgroup_hierarchy=0`
kernel parameter option to your boot command line, either [temporarily](
https://docs.fedoraproject.org/en-US/fedora/rawhide/system-administrators-guide/kernel-module-driver-configuration/Working_with_the_GRUB_2_Boot_Loader/#sec-Making_Temporary_Changes_to_a_GRUB_2_Menu)
or [permanently](https://fedoramagazine.org/setting-kernel-command-line-arguments-with-fedora-30/).
:::

In order to run Docker inside an nsbox container, you need two things:

- [Virtual networking.](docs.md#virtual-networking)
- A system call filter that allows kernel keyring access.

Both of these can be accomplished with a single config command:

```bash
$ nsbox-edge config -virtual-network -syscall-filters=':@default,@keyring' my-container
```

## Per-container VPNs

Similarly to the above, you can use virtual networking to get VPN connections that are active
per-container. Let's say you want to use OpenVPN. You can set up OpenVPN to run as a system
service on startup and if it were on the host:

```bash
$ sudo cp my-config.conf /etc/openvpn/client.conf
# Setup password auth as usual
$ systemctl enable --now openvpn-client@client
```

provided you make sure the container is configured with:

```bash
$ nsbox-edge config -virtual-network my-container
```

In addition, you'll need some browser installed inside it, such as Firefox, which can then
be exported onto the host:

```bash
$ nsbox-edge config -xdg-desktop-exports='+firefox'
```

Now, whenever you open the VPN container's Firefox, it'll automatically be running in a
container-local VPN.
