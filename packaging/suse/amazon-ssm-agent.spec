#
# spec file for package amazon-ssm-agent
#
# Copyright (c) 2017 SUSE LINUX GmbH, Nuernberg, Germany.
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via http://bugs.opensuse.org/
#

Name:           amazon-ssm-agent
Version:        2.0.672.0
Release:        0
License:        Apache-2.0
Summary:        Amazon Remote System Config Management
Url:            https://github.com/aws/amazon-ssm-agent
Group:          System/Management
Source0:        %{name}-%{version}.tar.gz
Source1:        %{name}.service
Patch1:         fix-version.patch
BuildRequires:  go >= 1.5
BuildRequires:  systemd
Requires:       systemd
Requires:       lsb-release
BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
This package provides the Amazon SSM Agent for managing EC2 Instances using
Amazon EC2 Systems Manager (SSM).

The SSM Agent runs on EC2 or on-premise instances and enables you to quickly
and easily execute remote commands or scripts against one or more instances.
When you execute a command, the agent on the instance processes the document
and configures the instance as specified.

This collection of capabilities helps you automate management tasks such as
collecting system inventory, applying operating system (OS) patches, automating
the creation of Amazon Machine Images (AMIs), and configuring operating systems
(OSs) and applications at scale. Systems Manager works with managed instances:
Amazon EC2 instances, or servers and virtual machines (VMs) in your on-premises
environment that are configured for Systems Manager.

%prep
%setup -q -n %{name}-%{version}
%patch1 -p1

%build
rm -rf vendor/src/github.com/aws/aws-sdk-go/vendor/

mkdir -p src/github.com/aws/amazon-ssm-agent
mv Tools agent vendor makefile amazon-ssm-agent.json.template \
seelog_unix.xml packaging src/github.com/aws/amazon-ssm-agent/

PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent
GOPATH=${PKG_ROOT}/vendor:`pwd`
export GOPATH

go build -ldflags "-s -w" -o bin/amazon-ssm-agent -v \
${PKG_ROOT}/agent/agent.go \
${PKG_ROOT}/agent/agent_unix.go \
${PKG_ROOT}/agent/agent_parser.go

%install
install -d -m 755 %{buildroot}%{_sbindir}
install -d -m 755 %{buildroot}%{_sysconfdir}/init
install -d -m 755 %{buildroot}%{_sysconfdir}/amazon/ssm
install -m 755 bin/amazon-ssm-agent %{buildroot}%{_sbindir}

mkdir -p %{buildroot}%{_unitdir}
install -m 755 %SOURCE1 %{buildroot}%{_unitdir}

PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent
cp ${PKG_ROOT}/seelog_unix.xml %{buildroot}/etc/amazon/ssm/seelog.xml.template
cp ${PKG_ROOT}/amazon-ssm-agent.json.template %{buildroot}/etc/amazon/ssm/
cp ${PKG_ROOT}/packaging/suse/amazon-ssm-agent.conf %{buildroot}/etc/init/

%files
%defattr(-,root,root,-)
%dir %{_sysconfdir}/init
%dir %{_sysconfdir}/amazon
%dir %{_sysconfdir}/amazon/ssm
%doc CONTRIBUTING.md LICENSE NOTICE.md README.md
%config(noreplace) %{_sysconfdir}/init/amazon-ssm-agent.conf
%config(noreplace) %{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
%config(noreplace) %{_sysconfdir}/amazon/ssm/seelog.xml.template
%{_sbindir}/*
%{_unitdir}/%{name}.service

%pre
%service_add_pre %{name}.service

%preun
%service_del_preun %{name}.service

%post
%service_add_post %{name}.service

%postun
%service_del_postun %{name}.service

%changelog
