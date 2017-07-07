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

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
	echo "upstart detected"
	echo "Installing agent" 
	rpm -U amazon-ssm-agent.rpm

	if [ "$DO_REGISTER" = true ]; then
		/sbin/stop amazon-ssm-agent
		amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
	fi

	agentVersion=$(rpm -q --qf '%{VERSION}\n' amazon-ssm-agent)
	echo "Installed version: $agentVersion"
	echo "starting agent"
	/sbin/start amazon-ssm-agent
	echo "$(status amazon-ssm-agent)"
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
	if [[ "$(systemctl is-active amazon-ssm-agent)" == "active" ]]; then
		echo "-> Agent is running in the instance"
		echo "Stopping the agent"
		echo "$(systemctl stop amazon-ssm-agent)"
		echo "Agent stopped"
		echo "$(systemctl daemon-reload)"
		echo "Reload daemon"	
	else
		echo "-> Agent is not running in the instance"
	fi
		
	echo "Installing agent" 
	echo "$(rpm -U amazon-ssm-agent.rpm)"

	if [ "$DO_REGISTER" = true ]; then
		$(systemctl stop amazon-ssm-agent)
		amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
	fi

	echo "Starting agent"
	$(systemctl daemon-reload)
	$(systemctl start amazon-ssm-agent)
	echo "$(systemctl status amazon-ssm-agent)"
else
	echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms"
fi
