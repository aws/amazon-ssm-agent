Name         : amazon-ssm-agent
Version      : 1.2.252.0
Release      : 1%{?dist}
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : Apache License, Version 2.0
BuildArch    : x86_64
BuildRoot    : %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html
Source0      : %{name}-%{version}.tar.gz

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

BuildRequires: golang >= 1.5

%description
This package provides Amazon SSM Agent for managing EC2 Instances using SSM APIs

%prep

%setup -q

%build
cd ..
mkdir -p %{name}-%{version}-tmp
mv %{name}-%{version}/* %{name}-%{version}-tmp/
mkdir -p %{name}-%{version}/src/github.com/aws/amazon-ssm-agent
mv %{name}-%{version}-tmp/* %{name}-%{version}/src/github.com/aws/amazon-ssm-agent/
rm -rf %{name}-%{version}-tmp
cd %{name}-%{version}
PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent
GOPATH=${PKG_ROOT}/vendor:`pwd`
export GOPATH

GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ${PKG_ROOT}/bin/amazon-ssm-agent -v \
${PKG_ROOT}/agent/agent.go ${PKG_ROOT}/agent/agent_unix.go ${PKG_ROOT}/agent/agent_parser.go

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/bin/
mkdir -p %{buildroot}/etc/init/
mkdir -p %{buildroot}/etc/amazon/ssm/
mkdir -p %{buildroot}/var/lib/amazon/ssm/

PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent

cp ${PKG_ROOT}/bin/amazon-ssm-agent %{buildroot}/usr/bin/
cp ${PKG_ROOT}/bin/ssm-cli %{buildroot}/usr/bin/
cp ${PKG_ROOT}/seelog_unix.xml %{buildroot}/etc/amazon/ssm/seelog.xml.template
cp ${PKG_ROOT}/amazon-ssm-agent.json.template %{buildroot}/etc/amazon/ssm/
cp ${PKG_ROOT}/packaging/linux/amazon-ssm-agent.conf %{buildroot}/etc/init/

%files
%defattr(-,root,root,-)
/etc/amazon/ssm/amazon-ssm-agent.json.template
/etc/amazon/ssm/seelog.xml.template
/usr/bin/amazon-ssm-agent
/var/lib/amazon/ssm/

%config(noreplace) /etc/init/amazon-ssm-agent.conf

%post
if [ $1 -eq 1 ] ; then
    # Initial installation
    /sbin/start amazon-ssm-agent
fi

%preun
if [ $1 -eq 0 ] ; then
    /sbin/stop amazon-ssm-agent
    sleep 1
fi

%postun
if [ $1 -ge 1 ]; then
    # restart service after upgrade
    /sbin/restart amazon-ssm-agent
fi
