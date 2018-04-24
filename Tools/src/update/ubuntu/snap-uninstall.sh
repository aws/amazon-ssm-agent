#!/bin/bash
echo "Uninstalling snap pkg"

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

# echo "Checking if the agent is installed"
# uninstall the agent if it is present
if [[ "$(cat /proc/1/comm)" == "systemd" ]]; then
	if [[ "$(systemctl is-active amazon-ssm-agent)" == "active" ]]; then
		# echo "-> Agent is running in the instance"
		# echo "Stopping the agent"
		systemctl stop snap.amazon-ssm-agent.amazon-ssm-agent.service
		echo "Agent stopped"
	else
		echo "-> Agent is not running in the instance"
	fi
else
    echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms"
fi