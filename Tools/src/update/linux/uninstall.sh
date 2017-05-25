#!/bin/bash

s3path=$1

echo "Uninstalling Amazon-ssm-agent"

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
	echo "Checking if the agent is installed" 
	if [ "$(rpm -q amazon-ssm-agent)" != "package amazon-ssm-agent is not installed" ]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent" 	
		rpm --erase amazon-ssm-agent
		sleep 1
	else
		echo "-> Agent is not installed in this instance"
	fi
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
	echo "Checking if the agent is installed" 
	if [[ "$(systemctl status amazon-ssm-agent)" != *"Loaded: not-found"* ]]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent" 	
		rpm --erase amazon-ssm-agent
		sleep 1
	else
		echo "-> Agent is not installed in this instance"
	fi
else
	echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms"
fi