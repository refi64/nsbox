# Guide

## Installation

### Fedora

::: warning
On the Fedora 31 betas, [the default SELinux policies for systemd-machined and,
by extension, systemd-nspawn, are somewhat
broken](https://bugzilla.redhat.com/show_bug.cgi?id=1760146). To work around this, you can
manually apply the audit2allow-generated policies in bug report (somewhat preferred) or set
SELinux to permissive ([not that ideal](https://stopdisablingselinux.com/)).
:::

Testing builds based off of the master branch are available on COPR:

```bash
# For standard Fedora
$ sudo dnf copr enable refi64/nsbox-edge
$ sudo dnf install nsbox-edge
# For Silverblue
$ sudo curl -Lo /etc/yum.repos.d/_copr:copr.fedorainfracloud.org:refi64:nsbox-edge.repo \
  https://copr.fedorainfracloud.org/coprs/refi64/nsbox-edge/repo/fedora-$(rpm -E %fedora)/refi64-nsbox-edge-fedora-$(rpm -E %fedora).repo
$ rpm-ostree install nsbox-edge
```

The nsbox-edge package installs the nsbox binary as *nsbox-edge*, that way the package can
be installed alongside the (future) stable releases. If you want to be able to call it as just
*nsbox*, install the *nsbox-alias* package.

### Arch Linux

You can install the edge builds from the AUR (note that this automatically adds the nsbox
symlink, but it will be changed eventually):

```bash
$ yay -S nsbox-edge-git
```

### Other distros

Currently you must [build from source](https://github.com/refi64/nsbox).

## Creating and deleting containers

For now, nsbox only ships with support for a Fedora image, either version 30 or 31. (There is a
**highly experimental** Arch image available as well; you can use it via the `arch` image name.)

Creating a new container is easy:

```bash
$ nsbox-edge fedora:30 my-container-name
```

If you want the container to run its own systemd instance, pass `-boot`:

```bash
$ nsbox-edge -boot fedora:30 my-container-name
```

::: tip
You can create custom base images using the steps outlined on [the images page](images.md).
:::

Deleting a container is similar:

```bash
$ nsbox-edge delete my-container-name
```

This will ask you to confirm you want to delete the container; if you want to assume yes,
pass `-y`.

In addition, deleting a container will fail if it is currently running. A container must
be [killed](#killing-containers) before it can be deleted.

## Running containers

By default, nsbox will run [the "default" container](#the-default-container).

```bash
# Run the default container.
$ nsbox-edge run
# Equivalent to the above (- signifies the default).
$ nsbox-edge run -
# Run another container.
$ nsbox-edge run my-other-container
```

This will start the container if it's not already running and then open a login shell inside.

::: tip
If your current login shell is not found in the container, nsbox will default to using bash.
For example, if I'm a zsh user, any new containers will use bash because zsh isn't installed.
Once I install zsh inside the container, then entering the container will result in zsh being
used.
:::

You can also run custom commands:

```bash
# Run neofetch in the default container.
$ nsbox-edge run - neofetch
# Run neofetch in another container.
$ nsbox-edge run my-other-container neofetch
```

## Managing your containers

### The default container

nsbox has the concept of a "default" container, which is the container run by default when
*run* is invoked. By default, the first container you create will be the default container.
If you want to change it, run:

```bash
$ nsbox-edge set-default my-container
```

If you want to unset the default container (so that there is none set), pass `-` as the new
container name.

### Inspecting containers

You can list all your containers with `nsbox list` and inspect a container with `nsbox info`:

```bash
$ nsbox-edge list
test
test-boot
...
$ nsbox-edge info test
                Name: test
              Booted: no
 XDG desktop exports: virt-manager
   XDG desktop extra:
             Running: since Sat, 28 Sep 2019 14:09:42 CDT (2 days ago)
              Memory: 19 MB
$
```

## Killing containers

Containers can be killed via `nsbox kill`:

```bash
$ nsbox-edge kill test
```

By default `kill` will send systemd-nspawn's SIGPOWEROFF, which will ask the container leader
to kill all the processes. For a more aggressive kill, use `kill -signal=kill` or
`kill -signal=sigkill` (the two are equivalent).

## Exporting desktop files onto the host

If you install GUI apps inside your container, you may want to be able to access them from
your host as if they were host apps. You can manage this via `nsbox config -xdg-desktop-exports`
and `nsbox config -xdg-desktop-extra`.

`nsbox config` is responsible for managing a container's configuration. The two options of
interest, `-xdg-desktop-extra` and `-xdg-desktop-exports`, both manage lists.

There are three ways to specify each option: you can append to the list, remove items from the
list, or set the entire contents of the list:

```bash
# Set the xdg-desktop-exports list to four items: a, b, c, and d:
$ nsbox-edge config -xdg-desktop-exports=:a,b,c my-container
# Remove b and d from the xdg-desktop-exports list:
$ nsbox-edge config -xdg-desktop-exports=-b,d my-container
# Append e to the exports list:
$ nsbox-edge config -xdg-desktop-exports=+e my-container
# Set the entire exports list to three items: x, y, and z:
$ nsbox-edge config -xdg-desktop-exports=:x,y,z my-container
# Set the list to be empty.
$ nsbox-edge config -xdg-desktop-exports=: my-container
```

- There are three control characters: `+`, `-`, and `:`.
  - `+` will append a comma-seperated list of items to the current list.
  - `-` will remove a comma-seperated list of items from the current list.
  - `:` will set the contents of the list to a comma-seperated list of items.

Of course, `-xdg-desktop-exports` and `-xdg-desktop-extra` can be combined.

### -xdg-desktop-exports

`-xdg-desktop-exports` manages a set of file patterns to match the name of desktop files to be
exported (without the `.desktop` extension):

```bash
# Export virt-manager.desktop.
$ nsbox-edge config -xdg-desktop-exports='+virt-manager' my-container
# Export any desktop file starting with "gnome-".
$ nsbox-edge config -xdg-desktop-exports='+gnome-*' my-container
# Export all desktop files (because * matches any file name).
$ nsbox-edge config -xdg-desktop-exports=':*' my-container
```

### -xdg-desktop-extra

`-xdg-desktop-extra` simply specifies a list of extra directories to search for desktop files.
By default, `/usr/share/applications` and `/usr/local/share/applications` will be searched.

```bash
# Export any desktop files directly under /opt/MyPoorlyPackagedProprietaryApp.
$ nsbox-edge config -xdg-desktop-extra='+/opt/MyPoorlyPackagedProprietaryApp' my-container
```

## Custom authentication

nsbox sets up the user account inside the container to mimic your host account, including
using the same password. This is necessary for sudo inside the container to work. However,
there are two scenarios where this isn't viable:

- You may be on a system that uses a remote authentication service, e.g. SSSD, in which
  case nsbox can't read the shadow database entry for your user.
- You may want to use a different password for the container account.

In order to use a custom password for the container, run:

```bash
$ nsbox-edge config -auth=manual my-container
```

nsbox will ask you to enter the custom password. This will be applied to the container the
next time you run it. (If it's already running, you will have to kill it first.)

## Got issues?

Check out the [FAQ](faq.md) for solutions to some common problems you may encounter.
