%global debug_package %{nil}

Name:           tdns
Version:        0.1.9
Release:        1%{?dist}
Summary:        Caching DNS server with encryption and network-wide policy
License:        MIT
URL:            https://github.com/jfardello/tdns

Source0:        %{url}/releases/download/v%{version}/tdns_Linux_x86_64.tar.gz
Source1:        %{url}/releases/download/v%{version}/tdns_Linux_arm64.tar.gz
Source2:        %{url}/releases/download/v%{version}/tdns_%{version}_checksums.txt
Source10:       tdns.service
Source11:       tdns.sysusers
Source12:       tdns.tmpfiles
Source13:       README.packaging
Source99:       tdns-rpmlintrc

ExclusiveArch:  x86_64 aarch64
BuildRequires:  file
BuildRequires:  systemd-rpm-macros
%if 0%{?suse_version}
BuildRequires:  sysuser-tools
%sysusers_requires
%else
Requires(pre):  systemd
%endif
Requires:       systemd

%description
TDNS is a caching DNS server with encryption, network-wide domain filtering,
split DNS, a management API, and an embedded web interface.
This package installs the static binary published by the matching upstream
release tag; it does not rebuild TDNS from source.

%prep
%setup -q -c -T
%ifarch x86_64
%global tdns_archive tdns_Linux_x86_64.tar.gz
%global tdns_machine x86-64
cp %{SOURCE0} %{tdns_archive}
%endif
%ifarch aarch64
%global tdns_archive tdns_Linux_arm64.tar.gz
%global tdns_machine ARM aarch64
cp %{SOURCE1} %{tdns_archive}
%endif

grep -F "  %{tdns_archive}" %{SOURCE2} > selected.checksum
test "$(wc -l < selected.checksum)" -eq 1
sha256sum --check --status selected.checksum
tar -xzf %{tdns_archive}
cp %{SOURCE13} README.packaging
test -x tdns
file -b tdns | grep -Fq "%{tdns_machine}"
file -b tdns | grep -Eq "statically linked.*stripped"
./tdns --version | grep -Fq "tdns version %{version}"

%build
%if 0%{?suse_version}
%sysusers_generate_pre %{SOURCE11} tdns
%endif

%install
install -D -m 0755 tdns %{buildroot}%{_bindir}/tdns
install -d -m 0755 %{buildroot}%{_mandir}/man1
./tdns man --output-dir %{buildroot}%{_mandir}/man1
install -D -m 0644 %{SOURCE10} %{buildroot}%{_unitdir}/tdns.service
install -D -m 0644 %{SOURCE11} %{buildroot}%{_sysusersdir}/tdns.conf
install -D -m 0644 %{SOURCE12} %{buildroot}%{_tmpfilesdir}/tdns.conf
install -d -m 0750 %{buildroot}%{_sysconfdir}/tdns
install -d -m 0750 %{buildroot}%{_sharedstatedir}/tdns

%if 0%{?suse_version}
%pre -f tdns.pre
%service_add_pre tdns.service
%endif

%post
%if 0%{?suse_version}
%tmpfiles_create %{_tmpfilesdir}/tdns.conf
%service_add_post tdns.service
%else
%tmpfiles_create %{_tmpfilesdir}/tdns.conf
%systemd_post tdns.service
%endif

%preun
%if 0%{?suse_version}
%service_del_preun tdns.service
%else
%systemd_preun tdns.service
%endif

%postun
%if 0%{?suse_version}
%service_del_postun tdns.service
%else
%systemd_postun_with_restart tdns.service
%endif

%files
%license LICENSE
%doc README.md README.packaging
%{_bindir}/tdns
%{_mandir}/man1/tdns*.1*
%{_unitdir}/tdns.service
%{_sysusersdir}/tdns.conf
%{_tmpfilesdir}/tdns.conf
%dir %attr(0750,root,tdns) %{_sysconfdir}/tdns
%dir %attr(0750,tdns,tdns) %{_sharedstatedir}/tdns

%changelog
* Sun Aug 09 2026 Jose Fardello <jmfardello@gmail.com> - 0.1.9-1
- Add the initial openSUSE and Fedora binary package recipe.
- Install hardened systemd, sysusers, and tmpfiles assets.
- Generate and install manual pages from the TDNS command tree.
