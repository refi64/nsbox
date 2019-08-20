# nsbox

nsbox is a lightweight, systemd-nspawn-powered pet container manager, aiming at supporting:

- Lightweight pet containers, integrated with the host (like the rootless toolbox script).
- machined-style booted containers.

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

The resulting files should all be under out/install.
