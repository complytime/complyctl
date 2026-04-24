# SPDX-License-Identifier: Apache-2.0

%global goipath github.com/complytime/complyctl
%global base_url https://%{goipath}
%global app_dir complytime
%global debug_package %{nil}

Name:           complyctl
Version:        0.0.8
Release:        1%{?dist}
Summary:        Compliance scanning CLI for OSCAL-based assessment workflows
License:        Apache-2.0
URL:            %{base_url}
Source0:        %{base_url}/archive/refs/tags/v%{version}.tar.gz

BuildRequires:  golang
BuildRequires:  go-rpm-macros

%gometa -f

%description
%{name} fetches Gemara policies from OCI registries, resolves dependency
graphs, dispatches scans to providers via gRPC, and produces compliance
reports (EvaluationLog, OSCAL, Markdown, SARIF).

Providers are distributed separately via the complytime-providers package.

%prep
%goprep -k

%build
BUILD_DATE_GO=$(date -u +'%%Y-%%m-%%dT%%H:%%M:%%SZ')

# Set up environment variables and flags to build properly and securely
%set_build_flags

# Align GIT_COMMIT and GIT_TAG with version for simplicity
GO_LD_EXTRAFLAGS="-X %{goipath}/internal/version.version=%{version} \
                  -X %{goipath}/internal/version.gitTreeState=clean \
                  -X %{goipath}/internal/version.commit=%{version} \
                  -X %{goipath}/internal/version.buildDate=${BUILD_DATE_GO}"

# Adapt go env to RPM build environment
export GO111MODULE=on

# Define and create the output directory for binaries
GO_BUILD_BINDIR=./bin
mkdir -p ${GO_BUILD_BINDIR}

# Build only the complyctl binary
go build -buildmode=pie -o ${GO_BUILD_BINDIR}/complyctl -ldflags="${GO_LD_EXTRAFLAGS}" ./cmd/complyctl

%install
install -d %{buildroot}%{_bindir}
install -d -m 0755 %{buildroot}%{_libexecdir}/%{app_dir}/providers
install -d %{buildroot}%{_mandir}/man1

install -p -m 0755 bin/complyctl %{buildroot}%{_bindir}/complyctl
install -p -m 0644 docs/man/complyctl.1 %{buildroot}%{_mandir}/man1/complyctl.1

%check
# Run unit tests
go test -mod=vendor -race -v ./...

%files
%attr(0755, root, root) %{_bindir}/complyctl
%{_mandir}/man1/complyctl.1*
%license LICENSE vendor/modules.txt
%doc README.md
%dir %{_libexecdir}/%{app_dir}
%dir %{_libexecdir}/%{app_dir}/providers

%changelog
* Fri Apr 24 2026 Marcus Burghardt <maburgha@redhat.com> - 0.0.8-1
- Simplify spec for core-only delivery after provider split
- Remove openscap-provider sub-package (moved to complytime-providers)
- Add complyctl.1 man page
- Add vendor/modules.txt for automatic bundled provides generation
- Build only complyctl binary from cmd/complyctl

* Wed Jul 9 2025 Marcus Burghardt <maburgha@redhat.com> - 0.0.8-1
- Bump to upstream version v0.0.8

* Tue Jul 8 2025 Marcus Burghardt <maburgha@redhat.com> - 0.0.7-1
- Bump to upstream version v0.0.7
- Include manifest file for openscap-plugin

* Mon Jun 16 2025 George Vauter <gvauter@redhat.com> - 0.0.6-2
- Update package name to complyctl

* Wed Jun 11 2025 Marcus Burghardt <maburgha@redhat.com> - 0.0.6-1
- Bump to upstream version v0.0.6
- Align with Fedora Package Guidelines

* Tue May 6 2025 Qingmin Duanmu <qduanmu@redhat.com> - 0.0.3-2
- Add complytime and openscap plugin man pages

* Wed Apr 30 2025 Qingmin Duanmu <qduanmu@redhat.com> - 0.0.3-1
- Separate plugin binary from manifest

* Fri Apr 11 2025 Qingmin Duanmu <qduanmu@redhat.com> - 0.0.2-1
- Separate package for openscap-plugin

* Tue Apr 08 2025 Marcus Burghardt <maburgha@redhat.com> - 0.0.2-1
- Initial RPM
