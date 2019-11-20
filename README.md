# nsbox

nsbox is a multi-purpose, nspawn-powered container manager. Please see the
[website](https://nsbox.dev) for more user-friendly information and documentation.

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

### Updating the theme

```bash
$ git -C VUEPRESS/packages/@vuepress/theme-default diff --relative v.PREV ':(exclude)__tests__' |\
  git apply --reject --directory web/.vuepress/theme
```
