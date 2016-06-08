#!/bin/bash

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

# check parameters for registering managed instance
DO_REGISTER=false
if [ "$1" == "register-managed-instance" ]; then
	if [ $# -eq 4 ]; then
		DO_REGISTER=true
		RMI_CODE=$2
		RMI_ID=$3
		RMI_REGION=$4
	else
		error_exit '[ERROR] Not enough parameters for RegisterManagedInstance.'
	fi
fi

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

if [ "$DO_REGISTER" = true ]; then
	/sbin/stop amazon-ssm-agent
	amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
fi

agentVersion=$(dpkg -s amazon-ssm-agent | grep 'Version')
echo "Installed version"

echo "starting agent"
/sbin/start amazon-ssm-agent

echo "$(status amazon-ssm-agent)"
