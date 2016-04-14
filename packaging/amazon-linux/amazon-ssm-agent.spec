Name         : amazon-ssm-agent
Version      : 1.1.145
Release      : 1%{?dist}
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : Amazon Software License
BuildArch    : x86_64
BuildRoot    : %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html
Source0      : %{name}-%{version}.tar.gz

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

BuildRequires: golang >= 1.3

%description
This package provides the Amazon SSM Agent for managing EC2 Instances using SSM APIs

%prep

%setup -q

%build
PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent
GOPATH=${PKG_ROOT}/vendor:`pwd`
export GOPATH

GOOS=linux GOARCH=amd64 go build -o ${PKG_ROOT}/bin/amazon-ssm-agent -v -x ${PKG_ROOT}/agent/agent.go

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/bin/
mkdir -p %{buildroot}/etc/init/
mkdir -p %{buildroot}/etc/amazon/ssm/
mkdir -p %{buildroot}/var/lib/amazon/ssm/

PKG_ROOT=`pwd`/src/github.com/aws/amazon-ssm-agent

cp ${PKG_ROOT}/bin/amazon-ssm-agent %{buildroot}/usr/bin/
cp ${PKG_ROOT}/seelog.xml %{buildroot}/etc/amazon/ssm/
cp ${PKG_ROOT}/amazon-ssm-agent.json %{buildroot}/etc/amazon/ssm/
cp ${PKG_ROOT}/packaging/amazon-linux-ami/amazon-ssm-agent.conf %{buildroot}/etc/init/

%files
%defattr(-,root,root,-)
/etc/init/amazon-ssm-agent.conf
/etc/amazon/ssm/amazon-ssm-agent.json
/etc/amazon/ssm/seelog.xml
/usr/bin/amazon-ssm-agent
/var/lib/amazon/ssm/

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