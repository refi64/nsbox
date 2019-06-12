#!/usr/bin/env python3

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

from __future__ import annotations

from typing import *

from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path

import argparse
import contextlib
import dataclasses
import fcntl
import grp
import json
import os
import pwd
import re
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request


# XXX: Find out why os.PathLike doesn't work out here.
MaybePath = Union[str, Path]


INDEX = 'https://dl.fedoraproject.org/pub/fedora/linux/releases/{version}/Container/x86_64/images'
DATA_DIR = Path('/var/lib/nsbox')
DATA_STORAGE_DIR = DATA_DIR / 'storage'


@dataclass(frozen=True)
class Userdata:
    user: str
    uid: int
    home: Path
    shell: Path
    groups: List[int]
    environ: Dict[str, str]

    @staticmethod
    def for_sudo_user() -> Userdata:
        assert os.getuid() == 0
        uid_str = os.environ.get('SUDO_UID') or os.environ.get('PKEXEC_UID')
        uid = int(uid_str) if uid_str else os.getuid()
        return Userdata.for_user(uid)

    @staticmethod
    def for_user(uid: Optional[int] = None) -> Userdata:
        if uid is None:
            uid = os.getuid()

        pw = pwd.getpwuid(uid)
        user = pw.pw_gecos
        home = Path(pw.pw_dir)
        shell = Path(pw.pw_shell)
        groups = [g.gr_gid for g in grp.getgrall() if user in g.gr_mem]

        return Userdata(
            user=user,
            uid=uid,
            home=home,
            shell=shell,
            groups=groups,
            environ=os.environ.copy(),
        )

    def to_environ_json(self) -> str:
        return json.dumps(self.environ)

    def with_environ_json(self, data: str) -> Userdata:
        environ = json.loads(data)
        return dataclasses.replace(self, environ=environ)


def get_default_version() -> Optional[int]:
    with open('/etc/os-release') as fp:
        for line in fp:
            if line.startswith('VERSION_ID='):
                return int(line.split('=')[1])

    sys.exit('Failed to find default distro version.')


def run(*args: MaybePath, cwd: Optional[MaybePath] = None,
        capture: bool = True) -> subprocess.CompletedProcess:
    text: Optional[bool] = None
    stdout: Optional[int] = None
    if capture:
        text = True
        stdout = subprocess.PIPE
    return subprocess.run(list(map(str, args)), cwd=str(cwd) if cwd else None, stdout=stdout,
                          text=text, check=True)


@contextlib.contextmanager
def locked_tmpdir() -> Iterator[Path]:
    with tempfile.TemporaryDirectory() as tmp:
        fd = os.open(tmp, os.O_DIRECTORY)
        try:
            fcntl.flock(fd, fcntl.LOCK_SH)
            try:
                yield Path(tmp)
            finally:
                fcntl.flock(fd, fcntl.LOCK_UN)
        finally:
            os.close(fd)


class IndexFilenameFound(Exception):
    def __init__(self, filename: str) -> None:
        super(IndexFilenameFound, self).__init__()
        self.filename = filename


class IndexHtmlParser(HTMLParser):
    def __init__(self) -> None:
        super(IndexHtmlParser, self).__init__()

        self.filename: Optional[str] = None

    def handle_starttag(self, tag: str, attrs: List[Tuple[str, str]]) -> None:
        attr_dict = dict(attrs)
        if tag == 'a' and attr_dict.get('href', '').startswith('Fedora-Container-Base'):
            raise IndexFilenameFound(attr_dict['href'])

    @staticmethod
    def find_filename(index: str) -> str:
        parser = IndexHtmlParser()
        try:
            parser.feed(index)
        except IndexFilenameFound as ex:
            return ex.filename
        else:
            sys.exit('Failed to parse index html.')


def get_script(name: str = '') -> Path:
    script = Path(__file__).parent.absolute() / 'scripts' / name
    assert script.exists()
    return script


def require_root() -> None:
    if os.getuid():
        sys.exit('This must be run as root.')


def exec_create(userdata: Userdata, args: Any) -> None:
    container: str = args.container
    version: int = args.version

    dest = DATA_STORAGE_DIR / container
    if dest.exists():
        sys.exit(f'{container} exists.')

    if version is None:
        version = get_default_version()

    print('Downloading index...')
    index_url = INDEX.format(version=version)
    with urllib.request.urlopen(index_url) as fp:
        index_html = fp.read().decode()

    filename = IndexHtmlParser.find_filename(index_html)

    with locked_tmpdir() as tmp:
        print(f'Downloading {filename}...')
        run('curl', '-o', tmp / filename, f'{index_url}/{filename}')

        print(f'Extracting layer.tar...')
        run('tar', '--strip-components=1', '-xf', filename, '*/layer.tar', cwd=tmp)

        print('Creating machine...')
        dest.mkdir(exist_ok=True, parents=True)
        run('tar', '-xf', tmp / 'layer.tar', cwd=dest)

    print('Complete.')


class NspawnBuilder:
    def __init__(self) -> None:
        self._args = ['systemd-nspawn']

    @property
    def args(self) -> List[str]:
        return self._args

    def add_argument(self, arg: str) -> None:
        self._args.append(f'--{arg}')

    def add_command(self, *command: str) -> None:
        self._args.extend(command)

    def add_quiet(self) -> None:
        self.add_argument('quiet')

    def add_machine_directory(self, path: MaybePath) -> None:
        self.add_argument(f'directory={path}')

    def add_link_journal(self, value: str) -> None:
        self.add_argument(f'link-journal={value}')

    def add_hostname(self, hostname: str) -> None:
        self.add_argument(f'hostname={hostname}')

    def add_env(self, name: str, value: str) -> None:
        self.add_argument(f'setenv={name}={value}')

    def _escape_mount_path(self, path: MaybePath) -> str:
        return str(path).replace('\\', '\\\\').replace(':', r'\:')

    def add_bind(self, host: MaybePath, dest: Optional[MaybePath] = None, *,
                 recursive: bool = False) -> None:
        dest = self._escape_mount_path(dest or host)
        host = self._escape_mount_path(host)
        opts = 'rbind' if recursive else 'norbind'
        self.add_argument(f'bind={host}:{dest}:{opts}')

    def exec(self) -> None:
        print(' '.join(map(shlex.quote, self.args)))
        os.execvp('systemd-nspawn', self.args)


def exec_run(userdata: Userdata, args: Any) -> None:
    ENV_WHITELIST = [
        'COLORTERM',
        'DBUS_SESSION_BUS_ADDRESS'
        'DBUS_SYSTEM_BUS_ADDRESS',
        'DESKTOP_SESSION',
        'DISPLAY',
        'LANG',
        'SHELL',
        'SSH_AUTH_SOCK',
        'TERM',
        'TOOLBOX_PATH',
        'VTE_VERSION',
        'WAYLAND_DISPLAY',
        'XDG_CURRENT_DESKTOP',
        'XDG_DATA_DIRS',
        'XDG_MENU_PREFIX',
        'XDG_RUNTIME_DIR',
        'XDG_SEAT',
        'XDG_SESSION_DESKTOP',
        'XDG_SESSION_ID',
        'XDG_SESSION_TYPE',
        'XDG_VTNR',
    ]

    container: str = args.container

    dest = DATA_STORAGE_DIR / container
    if not dest.exists():
        sys.exit(f'{container} does not exist.')

    nspawn = NspawnBuilder()
    nspawn.add_quiet()
    nspawn.add_machine_directory(dest)
    nspawn.add_link_journal('host')

    priv_path = Path('/var/lib/.nsbox-priv')
    host_priv_path = dest / priv_path.relative_to('/')
    if not host_priv_path.exists():
        host_priv_path.mkdir(exist_ok=True, parents=True)
    nspawn.add_bind(host_priv_path, '/run/host/nsbox')

    scripts_dir = get_script()
    nspawn.add_bind(scripts_dir, priv_path / 'scripts')

    supplementary_groups_file = host_priv_path / 'supplementary-groups'
    with supplementary_groups_file.open('w') as fp:
        for gid in userdata.groups:
            print(f'::{gid}', file=fp)

    env_file = host_priv_path / 'env'
    with env_file.open('w') as fp:
        for key, value in userdata.environ.items():
            print(key)
            if key not in ENV_WHITELIST:
                continue

            print(f'export {key}={shlex.quote(value)}', file=fp)

    nspawn.add_bind('/var/lib/systemd/coredump')

    with open('/etc/machine-id') as fp:
        machine_id = next(fp).strip()
    nspawn.add_bind(f'/var/log/journal/{machine_id}')

    if 'XDG_RUNTIME_DIR' in userdata.environ:
        nspawn.add_bind(userdata.environ['XDG_RUNTIME_DIR'])

    if 'DBUS_SYSTEM_BUS_ADDRESS' in userdata.environ:
        nspawn.add_bind(userdata.environ['DBUS_SYSTEM_BUS_ADDRESS'])
    else:
        nspawn.add_bind('/run/dbus')

    if os.path.exists('/run/media'):
        nspawn.add_bind('/run/media')

    home_parent = userdata.home.parent
    if home_parent.is_symlink():
        # We have a symlink somewhere, bind it.
        resolved_home_parent = home_parent.resolve()
        nspawn.add_bind(resolved_home_parent, recursive=True)
        nspawn.add_env('NSBOX_HOME_LINK_NAME', str(home_parent))
        nspawn.add_env('NSBOX_HOME_LINK_TARGET', str(resolved_home_parent))
    else:
        nspawn.add_bind(userdata.home, recursive=True)

    nspawn.add_bind('/etc', '/run/host/etc')

    shell = userdata.shell
    if not (dest / shell.relative_to('/')).exists():
        shell = Path('/usr/bin/bash')

    mail = Path('/var/mail') / userdata.user
    if mail.exists():
        nspawn.add_bind(mail, priv_path / 'mail')

    exec = args.exec or [str(userdata.shell)]

    nspawn.add_env('NSBOX_USER', userdata.user)
    nspawn.add_env('NSBOX_UID', str(userdata.uid))
    nspawn.add_env('NSBOX_SHELL', str(userdata.shell))
    nspawn.add_env('NSBOX_CWD', os.getcwd())
    nspawn.add_env('NSBOX_HOST_MACHINE', machine_id)
    nspawn.add_env('NSBOX_CONTAINER', container)

    nspawn.add_command('/run/host/nsbox/scripts/nsbox-enter.sh', *exec)
    nspawn.exec()


@dataclass(frozen=True)
class ToolboxImportInfo:
    packages: Set[str] = dataclasses.field(default_factory=set)
    debuginfo: Set[str] = dataclasses.field(default_factory=set)
    copr: Set[str] = dataclasses.field(default_factory=set)
    rpmfusion: Set[str] = dataclasses.field(default_factory=set)
    manual_repos: Set[str] = dataclasses.field(default_factory=set)


def get_toolbox_import_info(userdata: Userdata, container: Optional[str]) -> ToolboxImportInfo:
    result = ToolboxImportInfo()

    xdg_runtime_dir = userdata.environ.get('XDG_RUNTIME_DIR', f'/run/user/{userdata.uid}')
    toolbox_run = ['sudo', f'--user={userdata.user}', f'XDG_RUNTIME_DIR={xdg_runtime_dir}',
                   'toolbox', 'run']
    if container is not None:
        toolbox_run.extend(('-c', container))

    print('Querying package list... ')

    dnf_installed = toolbox_run[:]
    dnf_installed.extend(('dnf', 'repoquery', '--qf', '%{name}', '--userinstalled'))

    dnf_installed_output: str = run(*dnf_installed, capture=True, cwd='/').stdout

    for line in dnf_installed_output.splitlines():
        if line.startswith('fedora-release-'):
            continue
        elif line.endswith('-debuginfo'):
            result.debuginfo.add(line[:-len('-debuginfo')])
        else:
            result.packages.add(line)

    print('Querying repo list...')

    dnf_repolist = toolbox_run[:]
    # XXX: should probably...actually use repolist
    dnf_repolist.extend(('/usr/bin/ls', '-1', '/etc/yum.repos.d'))

    dnf_repolist_output: str = run(*dnf_repolist, capture=True, cwd='/').stdout

    for line in dnf_repolist_output.splitlines():
        assert line.endswith('.repo'), line
        line = line.rsplit('.', 1)[0]

        if line.startswith('_copr:'):
            _, hub, name, project = line.split(':')
            result.copr.add(f'{hub}/{name}/{project}')
        elif line == 'fedora-multimedia':
            result.manual_repos.add('https://negativo17.org/repos/fedora-multimedia.repo')
        elif line.startswith('rpmfusion-'):
            kind = line.split('-', 2)[1]
            result.rpmfusion.add(kind)

    return result


def exec_import(userdata: Userdata, args: Any) -> None:
    source: Optional[str] = args.source
    target: str = args.target

    # XXX: Duplicated from exec_run.
    dest = DATA_STORAGE_DIR / target
    if not dest.exists():
        sys.exit(f'{target} does not exist.')

    nspawn = NspawnBuilder()
    nspawn.add_quiet()
    nspawn.add_machine_directory(dest)

    imported = get_toolbox_import_info(userdata, source)
    commands: List[str] = []

    safety_re = re.compile(r'[a-zA-Z0-9\-+_/:.]+$')
    for field in dataclasses.astuple(imported):
        for item in field:
            assert safety_re.match(item), item

    if imported.copr or imported.debuginfo:
        commands.append(f'dnf install -y dnf-plugins-core')

    for copr in imported.copr:
        commands.append(f'dnf copr enable -y {copr}')

    for repo in imported.manual_repos:
        commands.append(f'dnf config-manager -y --add-repo={repo}')

    for rpmfusion in imported.rpmfusion:
        commands.append(f'dnf install -y https://download1.rpmfusion.org/{rpmfusion}/fedora/'
                            f'rpmfusion-{rpmfusion}-release-$(rpm -E %fedora).noarch.rpm')

    if imported.debuginfo:
        imported.packages.add('dnf-command\(debuginfo-install\)')
    commands.append(f'dnf install -y {" ".join(imported.packages)}')

    commands.append(f'dnf debuginfo-install -y {" ".join(imported.debuginfo)}')

    nspawn.add_command('bash', '-xc', '; '.join(commands))
    nspawn.exec()


COMMANDS = {
    'create': exec_create,
    'run': exec_run,
    'import': exec_import,
}


def main() -> None:
    parser = argparse.ArgumentParser(description='''
        nsbox is a lightweight, root/sudo-based alternative to the rootless toolbox script,
        build on top of systemd-nspawn instead of podman. This gives it several advantages,
        such as fewer bugs, a more authentic host experience, and no need to ever recreate a
        container in order to take advantage of newer changes.
    ''')

    parser.add_argument('--userdata', help=argparse.SUPPRESS)

    subcommands = parser.add_subparsers(dest='command', required=True)

    create_command = subcommands.add_parser('create', help='Create a new container')
    create_command.add_argument('--container', '-c', default='toolbox',
                                help='The container name')
    create_command.add_argument('--version', type=int, help='The Fedora version to use')

    run_command = subcommands.add_parser('run', help='Run a command inside the container')
    run_command.add_argument('--container', '-c', default='toolbox', help='The container name')
    run_command.add_argument('exec', nargs=argparse.REMAINDER,
                             help='The command to run (default is your shell)')

    import_command = subcommands.add_parser('import',
                                            help='Import the packages from a rootless toolbox')
    import_command.add_argument('--source', '-s', help='The toolbox container name')
    import_command.add_argument('--target', '-t', default='toolbox',
                                help='The nsbox container name')

    args = parser.parse_args()

    if os.getuid() != 0:
        os.execvp('sudo', ['sudo', os.path.abspath(__file__), '--userdata',
                           Userdata.for_user().to_environ_json(), *sys.argv[1:]])
    else:
        userdata = Userdata.for_sudo_user()
        if args.userdata:
            userdata = userdata.with_environ_json(args.userdata)

    COMMANDS[args.command](userdata, args)

if __name__ == '__main__':
    main()
