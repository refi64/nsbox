# nsbox

nsbox is a multi-purpose, nspawn-powered container manager. Please see the
[website](https://nsbox.dev) for more user-friendly information and documentation.

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

### Using Gerrit

[GerritHub](http://gerrithub.io) is used for all contributions (the Gerrit repository is just `refi64/nsbox`).

If you're not familiar with Gerrit, there is a generic walkthrough
[here](https://gerrit-review.googlesource.com/Documentation/intro-gerrit-walkthrough.html),
where `gerrithost` is replaced with `review.gerrithub.io`. In addition,
[this guide](https://gerrit-review.googlesource.com/Documentation/intro-gerrit-walkthrough-github.html)
is available for users familiar with GitHub pull requests.

### Running the tests

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
