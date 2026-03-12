%global debug_package %{nil}
%define __requires_exclude_from .*
AutoReq      : no
AutoProv     : no

Name         : amazon-ssm-agent
Version      : %{_version}
Release      : 1.wrlinux
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : ASL 2.0
BuildRoot    : %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

# No BuildRequires — binaries are pre-built via cross-compilation
# No ExcludeArch — targeting x86_64 explicitly

%description
This package provides Amazon SSM Agent for managing EC2 Instances using SSM APIs.
Built for Wind River Linux LTS (SysVinit).

%prep
# Nothing to prepare — binaries are pre-built

%build
# Nothing to build — binaries are pre-built

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}
cp -a %{_stagedir}/. %{buildroot}/

%files
%defattr(-,root,root,-)
%{_prefix}/bin/amazon-ssm-agent
%{_prefix}/bin/ssm-agent-worker
%{_prefix}/bin/ssm-document-worker
%{_prefix}/bin/ssm-session-worker
%{_prefix}/bin/ssm-session-logger
%{_prefix}/bin/ssm-cli
%{_sysconfdir}/amazon/ssm/amazon-ssm-agent.json.template
%{_sysconfdir}/amazon/ssm/seelog.xml.template
%config(noreplace) %{_sysconfdir}/init.d/amazon-ssm-agent
%{_localstatedir}/lib/amazon/ssm/
%dir %{_localstatedir}/log/amazon/ssm/

%doc
%{_sysconfdir}/amazon/ssm/README.md
%{_sysconfdir}/amazon/ssm/RELEASENOTES.md
%{_sysconfdir}/amazon/ssm/NOTICE.md

%post
# First install
if [ $1 -eq 1 ]; then
    # Create machine-id if not present (needed for instance fingerprint)
    if [ ! -f /etc/machine-id ] && [ ! -f /var/lib/dbus/machine-id ]; then
        if command -v uuidgen >/dev/null 2>&1; then
            uuidgen | tr -d '-' | tr '[:upper:]' '[:lower:]' > /etc/machine-id
        fi
    fi

    # Enable service at boot via rc symlinks
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d amazon-ssm-agent defaults
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add amazon-ssm-agent
    else
        # Manual symlinks as fallback
        for rl in 3 5; do
            ln -sf /etc/init.d/amazon-ssm-agent /etc/rc${rl}.d/S99amazon-ssm-agent 2>/dev/null || :
        done
        for rl in 0 1 6; do
            ln -sf /etc/init.d/amazon-ssm-agent /etc/rc${rl}.d/K01amazon-ssm-agent 2>/dev/null || :
        done
    fi

    # Start the agent
    /etc/init.d/amazon-ssm-agent start || :
fi

# Upgrade
if [ $1 -eq 2 ]; then
    /etc/init.d/amazon-ssm-agent restart || :
fi

%preun
# Only on uninstall (not upgrade)
if [ $1 -eq 0 ]; then
    /etc/init.d/amazon-ssm-agent stop >/dev/null 2>&1 || :

    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d -f amazon-ssm-agent remove
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --del amazon-ssm-agent
    else
        rm -f /etc/rc*.d/*amazon-ssm-agent 2>/dev/null || :
    fi
fi

%postun
# Nothing extra needed for SysVinit
