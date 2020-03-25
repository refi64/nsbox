%global goipath github.com/refi64/nsbox

%define reldir() %{lua:\
  local arg = rpm.expand('%1')\
  local prefix = rpm.expand('%{_prefix}')\
  assert(arg:sub(1, prefix:len()) == prefix, "arg " .. arg .. " does not start with " .. prefix)\
  local result = arg:sub(prefix:len() + 1):gsub('^/', '')\
  print(result)}

%global relbindir %{reldir %{_bindir}}
%global rellibexecdir %{reldir %{_libexecdir}}
%global reldatadir %{reldir %{_datadir}}

# nsbox-host has missing build-ids due to being static.
%global _missing_build_ids_terminate_build 0
# Scripts in data/scripts intentionally use a hashbang of /bin/bash (not /usr/bin)
# because the scripts are run inside container OSs that may not have performed the /usr
# merge yet. Skip automatically converting those hashbangs to /usr/bin/bash.
%global __brp_mangle_shebangs_exclude_from .*\.sh

Name: @PRODUCT_NAME
Version: @VERSION
%if "%{name}" == "nsbox-edge"
Release: 1%{?dist}.@COMMIT
%else
Release: 1%{?dist}
%endif
Summary: A multi-purpose, nspawn-powered container manager
License: MPL-2.0
URL: https://nsbox.dev/
BuildRequires: gcc
BuildRequires: gn
BuildRequires: go-rpm-macros
BuildRequires: golang
BuildRequires: ninja-build
BuildRequires: python3
BuildRequires: selinux-policy-devel
BuildRequires: systemd-devel
Requires: container-selinux
Requires: %{name}-selinux == %{version}-%{release}
Requires: polkit
Requires: sudo
Requires: systemd-container
Source0: nsbox-sources.tar

%description
nsbox is a multi-purpose, nspawn-powered container manager.

%package selinux
BuildArch: noarch
Summary: SELinux policy for %{name}
%{?selinux_requires}
%description selinux
This is the SELinux policy for %{name}.

%package bender
Summary: Build images for nsbox
Requires: ansible-bender
Requires: podman
Requires: python3
%description bender
nsbox-bender is a script built on top of ansible-bender to build base images for your nsbox
containers.

%if "%{name}" == "nsbox-edge"

%package alias
Summary: Alias for nsbox-edge
%description alias
Installs the nsbox alias for nsbox-edge.

%package bender-alias
Summary: Alias for nsbox-edge-bender
%description bender-alias
Installs the nsbox-bender alias for nsbox-edge-bender.

%endif

%prep
%setup -q

# @@ is here for substitute_file.py.
cat >build/go-shim.sh <<'EOF'
#!/bin/sh
if [[ "$1" == "build" ]]; then
  shift
  %gobuild "$@@"
else
  go "$@@"
fi
EOF

sed -i 's/GO111MODULE=off//g' build/go-shim.sh
chmod +x build/go-shim.sh

%build
%set_build_flags
unset LDFLAGS

mkdir -p out
cat >out/args.gn <<EOF
go_exe = "$PWD/build/go-shim.sh"
prefix = "%{_prefix}"
bin_dir = "%{relbindir}"
libexec_dir = "%{rellibexecdir}"
share_dir = "%{reldatadir}"
state_dir = "%{_sharedstatedir}"
config_dir = "%{_sysconfdir}"
enable_selinux = true
override_release_version = "@VERSION"
%if "%{name}" != "nsbox-edge"
is_stable_build = true
%endif
EOF

gn gen out
ninja -C out

%install
mkdir -p %{buildroot}/%{_prefix}
cp -r out/install/%{_sysconfdir} %{buildroot}
cp -r out/install/{%{relbindir},%{rellibexecdir},%{reldatadir}} %{buildroot}/%{_prefix}
chmod -R g-w %{buildroot}

%pre selinux
%selinux_relabel_pre

%post selinux
%selinux_modules_install %{_datadir}/selinux/packages/%{name}.pp.bz2

%postun selinux
if [ $1 -eq 0 ]; then
  %selinux_modules_uninstall %{name}
fi

%posttrans selinux
%selinux_relabel_post

%files
%{_bindir}/%{name}
%{_sysconfdir}/profile.d/%{name}.sh
%{_libexecdir}/%{name}/nsboxd
%{_libexecdir}/%{name}/nsbox-invoker
%{_libexecdir}/%{name}/nsbox-host
%{_datadir}/%{name}/data/getty-override.conf
%{_datadir}/%{name}/data/wants-networkd.conf
%{_datadir}/%{name}/data/nsbox-container.target
%{_datadir}/%{name}/data/nsbox-init.service
%{_datadir}/%{name}/data/scripts/nsbox-apply-env.sh
%{_datadir}/%{name}/data/scripts/nsbox-enter-run.sh
%{_datadir}/%{name}/data/scripts/nsbox-enter-setup.sh
%{_datadir}/%{name}/data/scripts/nsbox-init.sh
%{_datadir}/%{name}/images/arch/Dockerfile
%{_datadir}/%{name}/images/arch/metadata.json
%{_datadir}/%{name}/images/arch/playbook.yaml
%{_datadir}/%{name}/images/arch/roles/main/tasks/main.yaml
%{_datadir}/%{name}/images/debian/Dockerfile
%{_datadir}/%{name}/images/debian/metadata.json
%{_datadir}/%{name}/images/debian/playbook.yaml
%{_datadir}/%{name}/images/debian/roles/main/tasks/main.yaml
%{_datadir}/%{name}/images/fedora/metadata.json
%{_datadir}/%{name}/images/fedora/playbook.yaml
%{_datadir}/%{name}/images/fedora/roles/main/tasks/main.yaml
%{_datadir}/%{name}/images/fedora/roles/main/templates/nsbox.repo
%{_datadir}/%{name}/release/VERSION
%{_datadir}/%{name}/release/BRANCH
%{_datadir}/polkit-1/actions/@RDNS_NAME.policy
%{_datadir}/polkit-1/rules.d/@RDNS_NAME.rules

%files selinux
%{_datadir}/selinux/packages/%{name}.pp.bz2

%files bender
%{_bindir}/%{name}-bender
%{_datadir}/%{name}/python/%{name}-bender.py*

%if "%{name}" == "nsbox-edge"

%files alias
%{_bindir}/nsbox

%files bender-alias
%{_bindir}/nsbox-bender

%endif
