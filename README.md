# nsbox

nsbox is a multi-purpose, nspawn-powered container manager. Please see the
[website](https://nsbox.dev) for more user-friendly information and documentation.

## Links

- [sr.ht project](https://sr.ht/~refi64/nsbox/)
- [GitHub Mirror](https://github.com/refi64/nsbox/) (will be decomissioned in the future)
- [Legacy ora.pm issues board](https://ora.pm/project/211667/kanban) (superceded by the
  sr.ht project, will be decomissioned in the future)

[![View issues](https://img.shields.io/badge/issues-here-yellow)](https://ora.pm/project/211667/kanban)

## Build dependencies

You need:

- [Google's GN](https://gn.googlesource.com/gn) to generate the build files. Building this
  from source is pretty simple, see the instructions on the site for more info.
- [Ninja](https://ninja-build.org/) to actually...build stuff.
- The [Go compiler](http://golang.org).
- GCC or Clang for compiling cgo code.
- Python 3, which is used to run some of the build scripts.
- The systemd development headers.

## Building the code

Run:

```bash
$ go mod vendor
$ gn gen out
$ ninja -C out
```

The resulting files should all be under out/install. Then, you can run
`build/install.py out` to install to /usr/local (or set `--prefix` and/or `--destdir`, with the
usual meanings).

### Build configuration

Run `gn args --list out` to see all the configuration arguments nsbox supports. You can use
these options to set the saved paths (e.g. the libexec directory) to your distro's preferred
locations.

## Building the website

Run:

```bash
$ cd web
$ yarn
# Run a development web server:
$ yarn run dev
# Build the production docs:
$ yarn run build
```

## Contributing

### Submitting Patches

Please see the [guide for submitting patches on
git.sr.ht](https://man.sr.ht/git.sr.ht/#sending-patches-upstream). (If you choose to use
`git send-email`, the patches should be sent to
[~refi64/nsbox-devel@lists.sr.ht](https://lists.sr.ht/~refi64/nsbox-devel).)

### Coding Guidelines

TODO

### Running the tests

**These are not currently functional!** I'm doing a major overhaul to the way tests work.

Unit testing is done by running [Expect](https://www.tcl.tk/man/expect5.31/expect.1.html) scripts inside
an isolated environment. **Do not run the tests on your host system, as they will modify your containers.**

Vagrant is used to manage the virtual environments (as a VM is required to test SELinux integration).
The libvirt provider is required.

Run:

```bash
vagrant up
```

to bring up and provision the box (this includes building and installing nsbox inside). Once that is complete,
you can run:

```bash
vagrant ssh -c /vagrant/tests/main.exp
```

to run the unit tests.

TODO: document test runner

## Misc. notes

### Updating the theme

```bash
$ git -C VUEPRESS/packages/@vuepress/theme-default diff --relative v.PREV ':(exclude)__tests__' |\
  git apply --reject --directory web/.vuepress/theme
```
