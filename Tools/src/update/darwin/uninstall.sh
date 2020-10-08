#!/bin/bash

s3path=$1

echo "Uninstalling Amazon-ssm-agent"

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

echo "Checking if the agent is installed"
launchctl list com.amazon.aws.ssm >/dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "-> Agent is installed in this instance"
    echo "Uninstalling the agent"
    launchctl unload -w /Library/LaunchDaemons/com.amazon.aws.ssm.plist
    echo "Agent stopped"

    rm -rf /opt/aws/ssm/bin
    rm /opt/aws/ssm/seelog.xml.template
    rm /opt/aws/ssm/amazon-ssm-agent.json.template
    rm /opt/aws/ssm/RELEASENOTES.md
    rm /opt/aws/ssm/README.md
    rm /opt/aws/ssm/NOTICE.md
    rm /Library/LaunchDaemons/com.amazon.aws.ssm.plist
else
    echo "-> Agent is not installed in this instance"
fi


pkgutil --pkg-info com.amazon.aws.ssm >/dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "Uninstall the Agent pkg"
    pkgutil --forget com.amazon.aws.ssm >/dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Agent uninstalled"
    fi
fi