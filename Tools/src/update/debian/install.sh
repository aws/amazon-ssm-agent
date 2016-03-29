#!/bin/bash

s3path=$1

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

# allow ssm-agent to finish it's work
sleep 2

if [ "$(dpkg -s amazon-ssm-agent | grep 'Status:')" != "Status: install ok installed" ]; then
	echo "-> Agent is installed in this instance"
	# stop the agent if it is running 
	echo "Checking if the agent is running"
	if [ "$(status amazon-ssm-agent)" != "amazon-ssm-agent stop/waiting" ]; then
		echo "-> Agent is running in the instance"
  		echo "Stopping the agent"
  		$(stop amazon-ssm-agent)
  		sleep 1
	fi
fi

echo "Installing agent" 
dpkg -i amazon-ssm-agent.deb

agentVersion=$(dpkg -s amazon-ssm-agent | grep 'Version')
echo "Installed version"

echo "starting agent"
/sbin/start amazon-ssm-agent

echo "$(status amazon-ssm-agent)"
