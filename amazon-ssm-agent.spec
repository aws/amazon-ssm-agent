Name         : amazon-ssm-agent
Version      : %rpmversion
Release      : 1%{?dist}
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : ASL 2.0
BuildArch    : x86_64
BuildRoot    : %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html
Source0      : https://github.com/aws/amazon-ssm-agent/%{name}-%{version}.tar.gz

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

BuildRequires: golang >= 1.7.4

%if 0%{?amzn}
%global go_platform linux
%global go_arch amd64
%endif

%description
This package provides Amazon SSM Agent for managing EC2 Instances using SSM APIs

%prep

%setup -q
sed -i -e 's#const[ \s]*Version.*#const Version = "%{version}"#g' agent/version/version.go

%build

export GOPATH=`pwd`/vendor:`pwd`
export GOOS=%{go_platform}
export GOARCH=%{go_arch}

ln -s `pwd` vendor/src/github.com/aws/amazon-ssm-agent
go build -ldflags "-s -w" -o bin/amazon-ssm-agent -v agent/agent.go agent/agent_unix.go agent/agent_parser.go
go build -ldflags "-s -w" -o bin/ssm-document-worker -v agent/framework/processor/executer/outofproc/worker/main.go
go build -ldflags "-s -w" -o bin/ssm-cli -v agent/cli-main/cli-main.go

%install

rm -rf %{buildroot}
mkdir -p %{buildroot}%{_sysconfdir}/amazon/ssm/ \
         %{buildroot}%{_sysconfdir}/init/ \
         %{buildroot}%{_sysconfdir}/systemd/system/ \
         %{buildroot}%{_prefix}/bin/ \
         %{buildroot}%{_localstatedir}/lib/amazon/ssm/ \
         %{buildroot}%{_localstatedir}/log/amazon/ssm/

cp {README.md,RELEASENOTES.md} %{buildroot}%{_sysconfdir}/amazon/ssm/
cp bin/{amazon-ssm-agent,ssm-document-worker,ssm-cli} %{buildroot}%{_prefix}/bin/
cp packaging/linux/amazon-ssm-agent.conf %{buildroot}%{_sysconfdir}/init/
cp amazon-ssm-agent.json.template %{buildroot}%{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
cp seelog_unix.xml %{buildroot}%{_sysconfdir}/amazon/ssm/seelog.xml.template

strip --strip-unneeded %{buildroot}%{_prefix}/bin/{amazon-ssm-agent,ssm-document-worker,ssm-cli}

%files
%defattr(-,root,root,-)
%{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
%{_sysconfdir}/amazon/ssm/seelog.xml.template
%{_sysconfdir}/amazon/ssm/README.md
%{_sysconfdir}/amazon/ssm/RELEASENOTES.md
%{_sysconfdir}/init/amazon-ssm-agent.conf
%{_prefix}/bin/amazon-ssm-agent
%{_prefix}/bin/ssm-document-worker
%{_prefix}/bin/ssm-cli
%{_localstatedir}/lib/amazon/ssm/

%ghost %{_localstatedir}/log/amazon/ssm/

%doc
%{_sysconfdir}/amazon/ssm/README.md
%{_sysconfdir}/amazon/ssm/RELEASENOTES.md

%config(noreplace) %{_sysconfdir}/init/amazon-ssm-agent.conf

%preun
if [ $1 -eq 0 ] ; then
    /sbin/stop amazon-ssm-agent &> /dev/null || :
    sleep 1
fi

%post
if [ $1 -eq 2 ] ; then
    if [[ $(/sbin/status amazon-ssm-agent) =~ "amazon-ssm-agent start" ]] ; then
        /sbin/stop amazon-ssm-agent &> /dev/null || :
        /sbin/start amazon-ssm-agent &> /dev/null || :
    fi
fi