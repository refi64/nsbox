# nsbox

nsbox is a multi-purpose, nspawn-powered container manager aiming at supporting:

- Lightweight pet containers, complete with host integration (like the rootless toolbox script).
- machined-style booted containers.
- "Upgradable" containers, in that any enhancements to nsbox won't require re-creating your
  containers.
- A plugin system for adding support for multiple distros.

## Current status

nsbox supports having a Fedora guest container. Host integration consists of basic support for
exporting desktop files (but not icons yet) to the host system, as well as containers having
access to X11, Wayland, and PulseAudio. polkit will be eventually used to run containers in
an unprivileged way; in the meantime, you can pass `--sudo` to use sudo over pkexec if you
tire of the authentication dialogs.

You can create fully booted containers (LXC-style) by passing `--boot` to `nsbox create`.

To see more TODO items, check out [the issue tracker](https://github.com/refi64/nsbox/issues).

## Building

You need [Google's GN](https://gn.googlesource.com/gn), which has pretty easy build instructions
and a guide on how to build. You also need Python 3, which is used to run some of the build
scripts.

Run:

```bash
$ go mod vendor
$ gn gen out
$ ninja -C out
```

The resulting files should all be under out/install. Then, you can run
`build/install.py out` to install to /usr/local (or set `--prefix` and/or `--destdir`, with the
usual meanings).

To configure the build (e.g. build the guest tools rpm, change the libexec dir path, etc),
use `gn args out --list` to list the args and `gn args out` to set them.
