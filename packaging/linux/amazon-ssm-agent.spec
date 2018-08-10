Name         : amazon-ssm-agent
Version      : %rpmversion
Release      : 1
Summary      : Manage EC2 Instances using SSM APIs

Group        : Amazon/Tools
License      : Apache License, Version 2.0
URL          : http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html

Packager     : Amazon.com, Inc. <http://aws.amazon.com>
Vendor       : Amazon.com

%description
This package provides Amazon SSM Agent for managing EC2 Instances using SSM APIs

%files
%defattr(-,root,root,-)
/etc/amazon/ssm/amazon-ssm-agent.json.template
/etc/amazon/ssm/seelog.xml.template
/usr/bin/amazon-ssm-agent
/usr/bin/ssm-cli
/usr/bin/ssm-document-worker
/usr/bin/ssm-session-worker
/usr/bin/ssm-session-logger
/var/lib/amazon/ssm/
%doc /etc/amazon/ssm/RELEASENOTES.md
%doc /etc/amazon/ssm/README.md

%config(noreplace) /etc/init/amazon-ssm-agent.conf
%config(noreplace) /etc/systemd/system/amazon-ssm-agent.service

# The scriptlets in %pre and %post are run before and after a package is installed.
# The scriptlets %preun and %postun are run before and after a package is uninstalled.
# The scriptlets %pretrans and %posttrans are run at start and end of a transaction.

# Examples for the scriptlets are run for clean install, uninstall and upgrade

# Clean install: %posttrans
# Uninstall:     %preun
# Upgrade:       %pre, %posttrans

%pre
# Stop the agent before the upgrade
if [ $1 -ge 2 ]; then
    /sbin/init --version &> stdout.txt
    if [[ `cat stdout.txt` =~ upstart ]]; then
        /sbin/stop amazon-ssm-agent
    elif [[ `systemctl` =~ -\.mount ]]; then
        systemctl stop amazon-ssm-agent
        systemctl daemon-reload
    fi
    rm stdout.txt
fi

%preun
# Stop the agent after uninstall
if [ $1 -eq 0 ] ; then
    /sbin/init --version &> stdout.txt
    if [[ `cat stdout.txt` =~ upstart ]]; then
        /sbin/stop amazon-ssm-agent
        sleep 1
    elif [[ `systemctl` =~ -\.mount ]]; then
        systemctl stop amazon-ssm-agent
        systemctl disable amazon-ssm-agent
        systemctl daemon-reload
    fi
    rm stdout.txt
fi

%posttrans
# Start the agent after initial install or upgrade
if [ $1 -ge 0 ]; then
    /sbin/init --version &> stdout.txt
    if [[ `cat stdout.txt` =~ upstart ]]; then
        /sbin/start amazon-ssm-agent
    elif [[ `systemctl` =~ -\.mount ]]; then
        systemctl enable amazon-ssm-agent
        systemctl start amazon-ssm-agent
        systemctl daemon-reload
    fi
    rm stdout.txt
fi

%clean
# rpmbuild deletes $buildroot after building, specifying clean section to make sure it is not deleted


