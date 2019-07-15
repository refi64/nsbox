#!/usr/bin/env python3

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

from __future__ import annotations

from typing import Any, Dict, Iterator, List, Optional, Set, Tuple, Type, TypeVar, Union

from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path

import argparse
import contextlib
import dataclasses
import dbus  # type: ignore
import enum
import fcntl
import grp
import json
import os
import pwd
import re
import select
import signal
import shlex
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import termios
import time
import tty
import types
import urllib.request


# XXX: Find out why os.PathLike doesn't work out here.
MaybePath = Union[str, Path]


INDEX = 'https://dl.fedoraproject.org/pub/fedora/linux/releases/{version}/Container/x86_64/images'
DATA_DIR = Path('/var/lib/nsbox')
DATA_STORAGE_DIR = DATA_DIR / 'storage'
IN_CONTAINER_PRIV_PATH = Path('/var/lib/.nsbox-priv')


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

    def handle_starttag(self, tag: str, attrs: List[Tuple[str, Optional[str]]]) -> None:
        attr_dict = dict(attrs)
        if tag == 'a':
            href = attr_dict.get('href', '')
            if href is not None and href.startswith('Fedora-Container-Base'):
                raise IndexFilenameFound(href)

    @staticmethod
    def find_filename(index: str) -> str:
        parser = IndexHtmlParser()
        try:
            parser.feed(index)
        except IndexFilenameFound as ex:
            return ex.filename
        else:
            sys.exit('Failed to parse index html.')


def get_scripts_dir() -> Path:
    scripts_dir = Path(__file__).resolve().parent / 'scripts'
    assert scripts_dir.exists()
    assert scripts_dir.is_dir()
    return scripts_dir


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


def debug_print_command(command: List[str]) -> None:
    print(' '.join(map(shlex.quote, command)))


class NspawnBuilder:
    def __init__(self) -> None:
        nspawn = shutil.which('systemd-nspawn')
        assert nspawn is not None
        self._args = [nspawn]

    @property
    def args(self) -> List[str]:
        return self._args

    def add_argument(self, arg: str) -> None:
        self._args.append(f'--{arg}')

    def add_command(self, *command: str) -> None:
        self._args.extend(command)

    def add_quiet(self) -> None:
        self.add_argument('quiet')

    def add_as_pid2(self) -> None:
        self.add_argument('as-pid2')

    def add_machine_directory(self, path: MaybePath) -> None:
        self.add_argument(f'directory={path}')

    def add_link_journal(self, value: str) -> None:
        self.add_argument(f'link-journal={value}')

    def add_machine_name(self, machine: str) -> None:
        self.add_argument(f'machine={machine}')

    def add_hostname(self, hostname: str) -> None:
        self.add_argument(f'hostname={hostname}')

    def _escape_mount_path(self, path: MaybePath) -> str:
        return str(path).replace('\\', '\\\\').replace(':', r'\:')

    def add_bind(self, host: MaybePath, dest: Optional[MaybePath] = None, *,
                 recursive: bool = False) -> None:
        dest = self._escape_mount_path(dest or host)
        host = self._escape_mount_path(host)
        opts = 'rbind' if recursive else 'norbind'
        self.add_argument(f'bind={host}:{dest}:{opts}')

    def exec(self) -> None:
        debug_print_command(self.args)
        os.execvp('systemd-nspawn', self.args)


_BusProxy_T = TypeVar('_BusProxy_T', bound='BusProxy')

class BusProxy:
    def __init__(self, bus: Any, obj: Any, iface: Any) -> None:
        self.bus = bus
        self.obj = obj
        self.iface = iface

    @property
    def properties(self) -> PropertyProxy:
        if isinstance(self, PropertyProxy):
            return self

        iface = dbus.Interface(self.obj, dbus_interface='org.freedesktop.DBus.Properties')
        return PropertyProxy(self.bus, self.obj, iface, self.iface.dbus_interface)

    @classmethod
    def _get_object(cls: Type[_BusProxy_T], bus: Any, service: str, obj_path: str,
                    iface_name: str) -> _BusProxy_T:
        obj = bus.get_object(service, obj_path)
        iface = dbus.Interface(obj, dbus_interface=iface_name)
        return cls(bus, obj, iface)


class PropertyProxy(BusProxy):
    def __init__(self, bus: Any, obj: Any, iface: Any, accessor_iface_name: str) -> None:
        super(PropertyProxy, self).__init__(bus, obj, iface)
        self.accessor_iface_name = accessor_iface_name

    def get(self, prop: str) -> Any:
        return self.iface.Get(self.accessor_iface_name, prop)


class SystemdManagerProxy(BusProxy):
    SERVICE = 'org.freedesktop.systemd1'
    OBJECT = '/org/freedesktop/systemd1'
    IFACE = 'org.freedesktop.systemd1.Manager'

    @staticmethod
    def get(bus: Any) -> SystemdManagerProxy:
        return SystemdManagerProxy._get_object(bus, SystemdManagerProxy.SERVICE,
                                               SystemdManagerProxy.OBJECT,
                                               SystemdManagerProxy.IFACE)

    class UnitStartMode(enum.Enum):
        REPLACE = enum.auto()
        FAIL = enum.auto()
        ISOLATE = enum.auto()
        IGNORE_DEPENDENCIES = enum.auto()
        IGNORE_REQUIREMENTS = enum.auto()

        def to_bus_string(self) -> str:
            return self.name.lower().replace('_', '-')

    def get_unit(self, unit: str) -> SystemdUnitProxy:
        return SystemdUnitProxy.get(self.bus, str(self.iface.GetUnit(unit)))

    def get_unit_or_none(self, unit: str) -> Optional[SystemdUnitProxy]:
        try:
            return self.get_unit(unit)
        except dbus.exceptions.DBusException as ex:
            if ex.get_dbus_name() == 'org.freedesktop.systemd1.NoSuchUnit':
                return None
            raise

    def reset_failed_unit(self, unit: str) -> None:
        self.iface.ResetFailedUnit(unit)

    def start_transient_unit(self, name: str, mode: UnitStartMode,
                             properties: Dict[str, Any]) -> None:
        self.iface.StartTransientUnit(name, mode.to_bus_string(), list(properties.items()), [])


class SystemdUnitProxy(BusProxy):
    SERVICE = SystemdManagerProxy.SERVICE
    IFACE = 'org.freedesktop.systemd1.Unit'

    @staticmethod
    def get(bus: Any, obj_path: str) -> SystemdUnitProxy:
        return SystemdUnitProxy._get_object(bus, SystemdUnitProxy.SERVICE, obj_path,
                                            SystemdUnitProxy.IFACE)


class MachinedManagerProxy(BusProxy):
    SERVICE = 'org.freedesktop.machine1'
    OBJECT = '/org/freedesktop/machine1'
    IFACE = 'org.freedesktop.machine1.Manager'

    @dataclasses.dataclass
    class Pty:
        fd: int
        path: Path

    @staticmethod
    def get(bus: Any) -> MachinedManagerProxy:
        return MachinedManagerProxy._get_object(bus, MachinedManagerProxy.SERVICE,
                                                MachinedManagerProxy.OBJECT,
                                                MachinedManagerProxy.IFACE)

    def get_machine(self, machine: str) -> MachinedMachineProxy:
        return MachinedMachineProxy.get(self.bus, self.iface.GetMachine(machine))

    def get_machine_or_none(self, machine: str) -> Optional[MachinedMachineProxy]:
        try:
            return self.get_machine(machine)
        except dbus.exceptions.DBusException as ex:
            if ex.get_dbus_name() == 'org.freedesktop.machine1.NoSuchMachine':
                return None
            raise

    def open_machine_pty(self, machine: str) -> Pty:
        fd, path = self.iface.OpenMachinePTY(machine)
        return MachinedManagerProxy.Pty(fd=fd.take(), path=Path(path))


class MachinedMachineProxy(BusProxy):
    SERVICE = MachinedManagerProxy.SERVICE
    IFACE = 'org.freedesktop.machine1.Machine'

    @staticmethod
    def get(bus: Any, obj_path: str) -> MachinedMachineProxy:
        return MachinedMachineProxy._get_object(bus, MachinedMachineProxy.SERVICE,
                                                obj_path, MachinedMachineProxy.IFACE)

    def get_leader(self) -> int:
        return int(self.properties.get('Leader'))


class Container:
    def __init__(self, name: str) -> None:
        self.name = name

    def get_shell(self, userdata: Userdata) -> Path:
        if (self.directory / userdata.shell.relative_to('/')).exists():
            return userdata.shell
        else:
            return Path('/usr/bin/bash')

    @property
    def directory(self) -> Path:
        return DATA_STORAGE_DIR / self.name

    def ensure_exists(self) -> None:
        if not self.directory.exists():
            sys.exit(f'{self.name} does not exist.')

    @property
    def host_priv_path(self) -> Path:
        return self.directory / IN_CONTAINER_PRIV_PATH.relative_to('/')


class CommandInterrupt(BaseException):
    'Used by start_container to tell when a command was interrupted.'
    pass


def start_container(systemd: SystemdManagerProxy, machined: MachinedManagerProxy,
                    userdata: Userdata, container: Container) -> MachinedMachineProxy:
    service_name = f'nsbox-{container.name}.service'
    if systemd.get_unit_or_none(service_name) is not None:
        systemd.reset_failed_unit(service_name)

    nspawn = NspawnBuilder()
    nspawn.add_quiet()
    nspawn.add_as_pid2()
    nspawn.add_machine_name(container.name)
    nspawn.add_hostname('toolbox')
    nspawn.add_machine_directory(container.directory)
    nspawn.add_link_journal('host')

    if not container.host_priv_path.exists():
        container.host_priv_path.mkdir(exist_ok=True, parents=True)
    nspawn.add_bind(container.host_priv_path, '/run/host/nsbox')

    start_notify = container.host_priv_path / 'start-notify'
    if start_notify.exists():
        start_notify.unlink()

    scripts_dir = get_scripts_dir()
    nspawn.add_bind(scripts_dir, IN_CONTAINER_PRIV_PATH / 'scripts')

    nspawn.add_bind('/var/lib/systemd/coredump')

    with open('/etc/machine-id') as machine_id_fp:
        machine_id = next(machine_id_fp).strip()
    nspawn.add_bind(f'/var/log/journal/{machine_id}')

    if 'XDG_RUNTIME_DIR' in userdata.environ:
        nspawn.add_bind(userdata.environ['XDG_RUNTIME_DIR'])

    if 'DBUS_SYSTEM_BUS_ADDRESS' in userdata.environ:
        nspawn.add_bind(userdata.environ['DBUS_SYSTEM_BUS_ADDRESS'])
    else:
        nspawn.add_bind('/run/dbus')

    if os.path.exists('/run/media'):
        nspawn.add_bind('/run/media')

    env: Dict[str, str] = {}

    home_parent = userdata.home.parent
    if home_parent.is_symlink():
        # We have a symlink somewhere, bind it.
        resolved_home_parent = home_parent.resolve()
        nspawn.add_bind(resolved_home_parent, recursive=True)
        env['NSBOX_HOME_LINK_NAME'] = str(home_parent)
        env['NSBOX_HOME_LINK_TARGET'] = str(resolved_home_parent)
    else:
        nspawn.add_bind(userdata.home, recursive=True)

    nspawn.add_bind('/mnt', '/mnt')
    nspawn.add_bind('/etc', '/run/host/etc')

    mail = Path('/var/mail') / userdata.user
    if mail.exists():
        nspawn.add_bind(mail, IN_CONTAINER_PRIV_PATH / 'mail')

    shell = container.get_shell(userdata)

    env['NSBOX_USER'] = userdata.user
    env['NSBOX_UID'] = str(userdata.uid)
    env['NSBOX_SHELL'] = str(shell)
    env['NSBOX_HOST_MACHINE'] = machine_id

    supplementary_groups_file = container.host_priv_path / 'supplementary-groups'
    with supplementary_groups_file.open('w') as supplementary_groups_fp:
        for gid in userdata.groups:
            print(f'::{gid}', file=supplementary_groups_fp)

    shared_env_file = container.host_priv_path / 'shared-env'
    with shared_env_file.open('w') as shared_env_fp:
        for key, value in env.items():
            print(f'export {key}={shlex.quote(value)}', file=shared_env_fp)

    nspawn.add_command('/run/host/nsbox/scripts/nsbox-init.sh')
    # TODO: proper logging
    debug_print_command(nspawn.args)

    exec_start = [(nspawn.args[0], nspawn.args, False)]
    systemd.start_transient_unit(service_name, SystemdManagerProxy.UnitStartMode.REPLACE,
                                 {'Description': f'nsbox {container.name}',
                                  'ExecStart': exec_start})

    start = time.time()
    # TODO: make this timeout configurable, or use inotify properly (or both).
    while start + 3 > time.time():
        if start_notify.exists():
            break
    else:
        sys.exit(f'Container never started, try `systemctl status {service_name}`')

    return machined.get_machine(container.name)


@contextlib.contextmanager
def raw_stdin() -> Iterator[None]:
    old_attr = None
    try:
        old_attr = termios.tcgetattr(sys.stdin.fileno())
    except termios.error as ex:
        print(ex)
    else:
        tty.setraw(sys.stdin.fileno())

    try:
        yield
    finally:
        if old_attr is not None:
            termios.tcsetattr(sys.stdin.fileno(), termios.TCSAFLUSH, old_attr)


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

    container_name: str = args.container
    container = Container(container_name)
    container.ensure_exists()

    bus = dbus.SystemBus()
    systemd = SystemdManagerProxy.get(bus)
    machined = MachinedManagerProxy.get(bus)
    machine = machined.get_machine_or_none(container.name)
    if machine is None:
        machine = start_container(systemd, machined, userdata, container)

    command: List[str] = ['nsenter', '-at', str(machine.get_leader()),
                          '/run/host/nsbox/scripts/nsbox-enter.sh']

    # XXX: Okay so PTY handling is a royal mess. Basically, there are a couple of requirements:
    # We want what's a tty to be transparent. If stdout is a tty but stdin isn't, that needs to
    # be preserved.

    # Now, nsenter doesn't give us a pty *at all*. Therefore, the plan of action is to ask
    # machined for a pty inside the machine, and let nsbox-enter know where to redirect what.

    # NOTE: Order here is important, nsbox-enter.sh assumes it.
    stdio = (sys.stdin, sys.stdout, sys.stderr)

    pty: Optional[MachinedManagerProxy.Pty] = None
    to_redirect: Dict[int, int] = {}

    if any(io.isatty() for io in stdio):
        pty = machined.open_machine_pty(container.name)

        if sys.stdin.isatty():
            to_redirect[sys.stdin.fileno()] = pty.fd

        if sys.stdout.isatty() or sys.stderr.isatty():
            # If stdout isn't a tty, we can properly redirect stderr to stderr,
            # but otherwise we can't really tell the difference.
            if not sys.stdout.isatty():
                to_redirect[pty.fd] = sys.stderr.fileno()
            else:
                to_redirect[pty.fd] = sys.stdout.fileno()

    # Pass the redirects for nsbox-enter.sh.
    for io in stdio:
        if io.isatty():
            assert pty is not None
            command.append(str(pty.path))
        else:
            command.append('')

    # Add our cwd.
    command.append(os.getcwd())

    # Add environment command.
    command.append('env')

    for key, value in userdata.environ.items():
        if key not in ENV_WHITELIST:
            continue

        command.append(f'{key}={value}')

    shell = container.get_shell(userdata)
    command.extend(args.exec or [str(shell), '-l'])

    # TODO: proper logging
    debug_print_command(command)

    # If we don't have a tty, we can just exec straight up. However, if we do have a tty, we
    # need to stay as the parent process to redirect everything.
    if pty is None:
        os.execvp(command[0], command)

    with raw_stdin(), select.epoll() as epoll:
        # 4mb buffer.
        BUFSIZE = 4 * 1024 * 1024

        for source, target in to_redirect.items():
            epoll.register(source, select.EPOLLIN)

        def sigchld_handler(sig: signal.Signals, frame: types.FrameType) -> None:
            if process.poll() is not None:
                raise CommandInterrupt()

        def sync_sizes() -> None:
            assert pty is not None
            size = shutil.get_terminal_size()
            size_struct = struct.pack('hhhh', size.lines, size.columns, 0, 0)
            fcntl.ioctl(pty.fd, termios.TIOCSWINSZ, size_struct)

        def sigwinch_handler(sig: signal.Signals, frame: types.FrameType) -> None:
            sync_sizes()

        signal.signal(signal.SIGCHLD, sigchld_handler)
        signal.signal(signal.SIGWINCH, sigwinch_handler)
        sync_sizes()

        process = subprocess.Popen(command)

        while True:
            sync_sizes()

            try:
                for fd, events in epoll.poll():
                    if events & select.EPOLLIN:
                        buf = os.read(fd, BUFSIZE)
                        os.write(to_redirect[fd], buf)
                    elif events:
                        # Likely error, could be a hangup.
                        if not (events & select.EPOLLHUP):
                            print(f'Error occurred on {fd} -> {to_redirect[fd]}',
                                  file=sys.stderr)
                        epoll.unregister(fd)

                        if fd != pty.fd:
                            # Send close events over to the pty.
                            os.close(pty.fd)

            except CommandInterrupt:
                signal.signal(signal.SIGCHLD, signal.SIG_DFL)
                signal.signal(signal.SIGWINCH, signal.SIG_DFL)

                for source, target in to_redirect.items():
                    if source == sys.stdin.fileno():
                        continue

                    # Write anything left over in our buffers.
                    try:
                        os.set_blocking(source, False)
                        buf = os.read(source, BUFSIZE)
                        os.write(target, buf)
                    except (IOError, BlockingIOError):
                        pass

                process.wait()
                if process.returncode < 0:
                    sys.exit(128 + -process.returncode)
                else:
                        sys.exit(process.returncode)


def exec_kill(userdata: Userdata, args: Any) -> None:
    container_name: str = args.container
    container = Container(container_name)
    container.ensure_exists()

    os.execvp('machinectl', ['machinectl', 'kill', f'--signal={args.signal}', container.name])


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
    target_name: str = args.target
    target = Container(target_name)

    target.ensure_exists()

    # XXX: Duplicated from exec_run.
    nspawn = NspawnBuilder()
    nspawn.add_quiet()
    nspawn.add_machine_directory(target.directory)

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
    'kill': exec_kill,
    'import': exec_import,
}


def main() -> None:
    userdata: Userdata
    is_root = False

    if os.getuid() == 0:
        userdata = Userdata.for_sudo_user()
        is_root = True
    else:
        userdata = Userdata.for_user()

    default_container = f'toolbox-{userdata.user}'

    parser = argparse.ArgumentParser(description='''
        nsbox is a lightweight, root/sudo-based alternative to the rootless toolbox script,
        build on top of systemd-nspawn instead of podman. This gives it several advantages,
        such as fewer bugs, a more authentic host experience, and no need to ever recreate a
        container in order to take advantage of newer changes.
    ''')

    parser.add_argument('--environ', help=argparse.SUPPRESS)

    subcommands = parser.add_subparsers(dest='command', required=True)

    create_command = subcommands.add_parser('create', help='Create a new container')
    create_command.add_argument('--container', '-c', default=default_container,
                                help='The container name')
    create_command.add_argument('--version', type=int, help='The Fedora version to use')

    run_command = subcommands.add_parser('run', help='Run a command inside the container')
    run_command.add_argument('--container', '-c', default=default_container,
                             help='The container name')
    run_command.add_argument('exec', nargs=argparse.REMAINDER,
                             help='The command to run (default is your shell)')

    kill_command = subcommands.add_parser('kill', help='Kill a container')
    kill_command.add_argument('--container', '-c', default=default_container,
                              help='The container name')
    kill_command.add_argument('--signal', '-s', default='SIGTERM',
                              help='The signal to kill with')

    import_command = subcommands.add_parser('import',
                                            help='Import the packages from a rootless toolbox')
    import_command.add_argument('--source', '-s', help='The toolbox container name')
    import_command.add_argument('--target', '-t', default=default_container,
                                help='The nsbox container name')

    args = parser.parse_args()

    if not is_root:
        os.execvp('sudo', ['sudo', os.path.abspath(__file__), '--environ',
                           userdata.to_environ_json(), *sys.argv[1:]])

    if args.environ:
        userdata = userdata.with_environ_json(args.environ)

    COMMANDS[args.command](userdata, args)

if __name__ == '__main__':
    main()
