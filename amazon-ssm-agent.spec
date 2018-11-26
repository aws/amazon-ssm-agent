Name         : amazon-ssm-agent
Version      : %rpmversion
Release      : 1%{?dist}
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : ASL 2.0
ExcludeArch  : %{ix86}
BuildRoot    : %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html
Source0      : https://github.com/aws/amazon-ssm-agent/%{name}-%{version}.tar.gz

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

BuildRequires: golang >= 1.7.4

%if 0%{?amzn} >= 2
BuildRequires: systemd-devel
Requires(post): systemd-units
Requires(preun): systemd-units
Requires(postun): systemd-units
%endif

%description
This package provides Amazon SSM Agent for managing EC2 Instances using SSM APIs

%prep

%setup -q
sed -i -e 's#const[ \s]*Version.*#const Version = "%{version}"#g' agent/version/version.go

%build

export GOPATH=`pwd`/vendor:`pwd`

ln -s `pwd` vendor/src/github.com/aws/amazon-ssm-agent
go build -ldflags "-s -w" -o bin/amazon-ssm-agent -v agent/agent.go agent/agent_unix.go agent/agent_parser.go
go build -ldflags "-s -w" -o bin/ssm-document-worker -v agent/framework/processor/executer/outofproc/worker/main.go
go build -ldflags "-s -w" -o bin/ssm-session-worker -v agent/framework/processor/executer/outofproc/sessionworker/main.go
go build -ldflags "-s -w" -o bin/ssm-session-logger -v agent/session/logging/main.go
go build -ldflags "-s -w" -o bin/ssm-cli -v agent/cli-main/cli-main.go

%install

rm -rf %{buildroot}
mkdir -p %{buildroot}%{_sysconfdir}/amazon/ssm/ \
         %{buildroot}%{_sysconfdir}/init/ \
         %{buildroot}%{_prefix}/bin/ \
         %{buildroot}%{_localstatedir}/lib/amazon/ssm/ \
         %{buildroot}%{_localstatedir}/log/amazon/ssm/

cp {README.md,RELEASENOTES.md} %{buildroot}%{_sysconfdir}/amazon/ssm/
cp bin/{amazon-ssm-agent,ssm-document-worker,ssm-session-worker,ssm-session-logger,ssm-cli} %{buildroot}%{_prefix}/bin/
%if 0%{?amzn} >= 2
mkdir -p %{buildroot}%{_unitdir}/
cp packaging/linux/amazon-ssm-agent.service %{buildroot}%{_unitdir}/
%else 
cp packaging/linux/amazon-ssm-agent.conf %{buildroot}%{_sysconfdir}/init/
%endif
cp amazon-ssm-agent.json.template %{buildroot}%{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
cp seelog_unix.xml %{buildroot}%{_sysconfdir}/amazon/ssm/seelog.xml.template

strip --strip-unneeded %{buildroot}%{_prefix}/bin/{amazon-ssm-agent,ssm-document-worker,ssm-session-worker,ssm-session-logger,ssm-cli}

%files
%defattr(-,root,root,-)
%{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
%{_sysconfdir}/amazon/ssm/seelog.xml.template
%{_sysconfdir}/amazon/ssm/README.md
%{_sysconfdir}/amazon/ssm/RELEASENOTES.md
%if 0%{?amzn} >= 2
%{_unitdir}/amazon-ssm-agent.service
%else
%{_sysconfdir}/init/amazon-ssm-agent.conf
%endif
%{_prefix}/bin/amazon-ssm-agent
%{_prefix}/bin/ssm-document-worker
%{_prefix}/bin/ssm-session-worker
%{_prefix}/bin/ssm-session-logger
%{_prefix}/bin/ssm-cli
%{_localstatedir}/lib/amazon/ssm/

%ghost %{_localstatedir}/log/amazon/ssm/

%doc
%{_sysconfdir}/amazon/ssm/README.md
%{_sysconfdir}/amazon/ssm/RELEASENOTES.md

%if 0%{?amzn} < 2
%config(noreplace) %{_sysconfdir}/init/amazon-ssm-agent.conf
%endif

%preun
%if 0%{?amzn} >= 2
%systemd_preun %{name}.service
%else
if [ $1 -eq 0 ] ; then
    /sbin/stop amazon-ssm-agent &> /dev/null || :
    sleep 1
fi
%endif

%post
%if 0%{?amzn} >= 2
%systemd_post %{name}.service
%else
if [ $1 -eq 2 ] ; then
    if [[ $(/sbin/status amazon-ssm-agent) =~ "amazon-ssm-agent start" ]] ; then
        /sbin/stop amazon-ssm-agent &> /dev/null || :
        /sbin/start amazon-ssm-agent &> /dev/null || :
    fi
fi
%endif

%postun
%if 0%{?amzn} >= 2
%systemd_postun_with_restart %{name}.service
%endif
