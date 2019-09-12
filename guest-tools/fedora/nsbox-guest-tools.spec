Name: nsbox-guest-tools
Version: @VERSION
Release: 1%{?dist}
Summary: Tools for nsbox host integration
License: MPL-2.0
URL: https://nsbox.dev/
BuildArch: noarch
Provides: dnf-plugin-nsbox = %{version}-%{release}
BuildRequires: python3-devel
Requires: python3-dnf
Source0: nsbox_trigger.py

%description
Guest tools for nsbox containers that allow integration with the host system.

%build
cp %{_sourcedir}/nsbox_trigger.py %{_builddir}
cd %{_builddir}
%{__python3} -m compileall nsbox_trigger.py
%{__python3} -O -m compileall nsbox_trigger.py

%install
install -Dm 644 -t %{buildroot}/%{python3_sitelib}/dnf-plugins %{_builddir}/nsbox_trigger.py
install -Dm 644 -t %{buildroot}/%{python3_sitelib}/dnf-plugins/__pycache__ %{_builddir}/__pycache__/nsbox_trigger.*.pyc
mkdir -p %{buildroot}/%{_bindir}
ln -s /run/host/nsbox/nsbox-host %{buildroot}/%{_bindir}/nsbox-host

%files
%{_bindir}/nsbox-host
%{python3_sitelib}/dnf-plugins/nsbox_trigger.py
%{python3_sitelib}/dnf-plugins/__pycache__/nsbox_trigger.*.pyc
