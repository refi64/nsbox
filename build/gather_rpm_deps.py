# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Parses go.mod and generates a list of rpm Requires/Source# statements and %setup
# commands.

from go_deps import go_list

import argparse
import collections
import contextlib
import os.path


FEDORA_REPO_AVAILABLE_DEPS = {
    'github.com/coreos/go-systemd',
    'github.com/dustin/go-humanize',
    'github.com/godbus/dbus',
    'github.com/kr/pty',
    'github.com/pkg/errors',
    'golang.org/x/crypto',
    'golang.org/x/sync',
    'golang.org/x/sys',
    'gopkg.in/alecthomas/kingpin.v2',
    'gopkg.in/cheggaaa/pb.v1',
}


MANUALLY_DOWNLOADED_DEPS = {
    'github.com/google/go-containerregistry',
    'github.com/google/subcommands',
    'github.com/refi64/go-lxtempdir',
    'github.com/varlink/go',
}

F31_ONLY_REPO_DEPS = {
    'github.com/google/go-cmp',

    # Too old in F30.
    'github.com/godbus/dbus',
    'github.com/kr/pty',
    'github.com/mholt/archiver',

    # ...and archiver's deps:
    'github.com/golang/snappy',
    'github.com/dsnet/compress',
    'github.com/nwaples/rardecode',
    'github.com/pierrec/lz4',
    'github.com/xi2/xz',
    'github.com/ulikunitz/xz',

    'k8s.io/apimachinery',
    'k8s.io/klog',
}

NSBOX_PACKAGE_NAME = 'github.com/refi64/nsbox'

Module = collections.namedtuple('Module', ['name', 'revision', 'indirect', 'child_deps'])

RepoSource = collections.namedtuple('RepoSource', ['url', 'imports', 'f31_only', 'indirect'])
VendorSource = collections.namedtuple('VendorSource', ['url', 'revision', 'pre_f31_only'])


# We don't want to count indirect deps of stuff available in repos, but we *do* need to count
# them for stuff that will be downloaded manually.
# XXX: This is kinda inefficient but mostly fine for our use cases.

class SourceGatherEngine:
    def __init__(self, deps):
        self._deps = deps
        self._pkg_sources = collections.defaultdict(list)

        self._unknown = set()

        self._modmap = {}
        self._importmap = {}

        self._build_modmap()

    def _build_modmap(self):
        self._importmap['C'] = None

        for dep in self._deps:
            if dep['ImportPath'].startswith(NSBOX_PACKAGE_NAME):
                continue

            if dep.get('Standard'):
                name = None
            else:
                module = dep['Module']
                name = module['Path']

                if name not in self._modmap:
                    if module['Version'].startswith('v0.0.0'):
                        revision = module['Version'].rsplit('-', 1)[-1]
                    else:
                        revision = module['Version']

                        incompat = '+incompatible'
                        if revision.endswith(incompat):
                            revision = revision[:-len(incompat)]

                    self._modmap[name] = Module(name=name, revision=revision,
                                                indirect=module.get('Indirect', False),
                                                child_deps=[])

                self._modmap[name].child_deps.append(dep)

            self._importmap[dep['ImportPath']] = name

    def _process_module(self, module, *, parent=None, f31_only_parent=False):
        if module.name in self._pkg_sources:
            return

        found = False

        if (module.name in FEDORA_REPO_AVAILABLE_DEPS | F31_ONLY_REPO_DEPS
            or (parent is not None
                and all(isinstance(src, RepoSource) for src in self._pkg_sources[parent.name]))):
            source = RepoSource(imports=[child['ImportPath'] for child in module.child_deps],
                                f31_only=module.name in F31_ONLY_REPO_DEPS,
                                indirect=module.indirect, url=module.name)
            self._pkg_sources[module.name].append(source)

            found = True

        if module.name in MANUALLY_DOWNLOADED_DEPS | F31_ONLY_REPO_DEPS:
            source = VendorSource(url=module.name,
                                  revision=module.revision,
                                  pre_f31_only=module.name in F31_ONLY_REPO_DEPS
                                                or f31_only_parent)
            self._pkg_sources[module.name].append(source)

            found = True

        if not found:
            self._unknown.add(module.name)
            return

        for child in module.child_deps:
            for imp in child.get('Imports', []):
                imp_module = self._importmap[imp]
                if imp_module is not None:
                    self._process_module(self._modmap[imp_module], parent=module,
                                         f31_only_parent=module.name in F31_ONLY_REPO_DEPS
                                                            or f31_only_parent)

    def _process_direct_modules(self):
        for module in self._modmap.values():
            if not module.indirect:
                self._process_module(module)

    @staticmethod
    def gather_sources(deps):
        engine = SourceGatherEngine(deps)
        engine._process_direct_modules()

        assert not engine._unknown, f'Unknown modules: {", ".join(engine._unknown)}'
        return engine._pkg_sources


class SpecGenerator:
    def __init__(self, fp, source_offset):
        self._fp = fp
        self._start_source_offset = self._source_offset = self._setup_offset = source_offset
        self._in_macro = False

    def _add(self, line):
        if self._in_macro:
            line += '\\'

        print(line, file=self._fp)

    @contextlib.contextmanager
    def _with_macro(self, name):
        assert not self._in_macro
        self._add('%define ' + name + ' \\')
        self._in_macro = True
        yield
        self._in_macro = False
        self._add(':')

    def _build_requires(self, req):
        self._add(f'BuildRequires: {req}')

    def _source(self, url):
        self._add(f'Source{self._source_offset}: {url}')
        self._source_offset += 1

    @contextlib.contextmanager
    def f31_only_if(self, f31_only):
        if f31_only:
            self._add('%if 0%{?fedora} >= 31')

        yield

        if f31_only:
            self._add('%endif')

    def _map_sorted_sources(self, sources, ty, func):
        for _, module_sources in sorted(sources.items(), key=lambda p: p[0]):
            for source in module_sources:
                if isinstance(source, ty):
                    func(source)

    def _generate_repo_source(self, source):
        if source.indirect:
            return

        with self.f31_only_if(source.f31_only):
            for imp in source.imports:
                self._build_requires(f'golang({imp})')

    def _setup_repo_source(self, source):
        self._add(f'mkdir -p vendor/{os.path.dirname(source.url)}')
        self._add(f'ln -sf %{{gopath}}/src/{source.url} vendor/{source.url}')

    def _add_vendor_source(self, source):
        if source.url.startswith('k8s.io'):
            url = 'github.com/kubernetes/' + source.url.split('/', 1)[1]
        else:
            url = source.url

        assert url.startswith('github.com'), source

        escaped_url = source.url.replace('/', '-').replace('.', '-')
        archive_name = f'{escaped_url}-{source.revision}.tar.gz'
        archive = f'https://{url}/archive/{source.revision}.tar.gz#/{archive_name}'
        self._source(archive)

    def _setup_vendor_source(self, source):
        self._add(f'%setup -q -T -c -n %{{name}}-%{{version}}/vendor/{source.url}')
        self._add(f'tar --strip-components=1 -xf %{{S:{self._setup_offset}}}')
        self._setup_offset += 1

    def _generate_universal_vendor_source(self, source):
        if not source.pre_f31_only:
            self._add_vendor_source(source)

    def _setup_universal_vendor_source(self, source):
        if not source.pre_f31_only:
            self._setup_vendor_source(source)

    def _generate_pre_f31_only_vendor_source(self, source):
        if source.pre_f31_only:
            self._add_vendor_source(source)

    def _setup_pre_f31_only_vendor_source(self, source):
        if source.pre_f31_only:
            self._setup_vendor_source(source)

    def _generate_for_sources(self, sources):
        self._map_sorted_sources(sources, RepoSource, self._generate_repo_source)

        with self._with_macro('setup_go_repo_links'):
            self._add('cd %{_builddir}/%{name}-%{version}')
            self._map_sorted_sources(sources, RepoSource, self._setup_repo_source)

        self._map_sorted_sources(sources, VendorSource, self._generate_universal_vendor_source)

        with self._with_macro('setup_go_archives_universal'):
            self._map_sorted_sources(sources, VendorSource, self._setup_universal_vendor_source)

        self._add('%if 0%{?fedora} < 31')
        self._map_sorted_sources(sources, VendorSource, self._generate_pre_f31_only_vendor_source)

        with self._with_macro('setup_go_archives_pre_f31_only'):
            self._map_sorted_sources(sources, VendorSource,
                                     self._setup_pre_f31_only_vendor_source)

        self._add('%endif')

    @staticmethod
    def generate(fp, source_offset, sources):
        gen = SpecGenerator(fp, source_offset)
        gen._generate_for_sources(sources)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--go')
    parser.add_argument('--output')
    parser.add_argument('--source-offset', type=int, default=0)
    args = parser.parse_args()

    deps = go_list(args.go, 'all')

    sources = SourceGatherEngine.gather_sources(deps)
    with open(args.output, 'w') as fp:
        SpecGenerator.generate(fp, args.source_offset, sources)


if __name__ == '__main__':
    main()
