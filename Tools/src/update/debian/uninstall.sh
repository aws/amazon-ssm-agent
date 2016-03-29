#!/bin/bash

s3path=$1

echo "Installing deb pkg"

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

echo "Checking if the agent is installed" 
# uninstall the agent if it is present
if [ "$(dpkg -s amazon-ssm-agent | grep 'Status:')" != "Status: install ok installed" ]; then
	echo "-> Agent is installed in this instance"
	# stop the agent if it is running 
	echo "Checking if the agent is running"
	if [ "$(status amazon-ssm-agent)" != "amazon-ssm-agent stop/waiting" ]; then
		echo "-> Agent is running in the instance"
  		echo "Stopping the agent"
  		$(stop amazon-ssm-agent)
  		sleep 1
	else
		echo "-> Agent is not running"
	fi
	echo "Uninstalling the agent" 	
	$(dpkg -r amazon-ssm-agent)
	sleep 1
else
	echo "-> Agent is not installed in this instance"
fi
