#!/bin/bash

s3path=$1

echo "Uninstalling deb pkg"

# helper function to set error output
function error_exit
{
	echo "$1" 1>&2
	exit 1
}

# echo "Checking if the agent is installed"
# uninstall the agent if it is present
if [[ "$(cat /proc/1/comm)" == "init" ]]; then
    if [ "$(dpkg -s amazon-ssm-agent | grep 'Status:')" == "Status: install ok installed" ]; then
        # echo "-> Agent is installed in this instance"
        # stop the agent if it is running
       #  echo "Checking if the agent is running"
        if [ "$(status amazon-ssm-agent)" != "amazon-ssm-agent stop/waiting" ]; then
            # echo "-> Agent is running in the instance"
            # echo "Stopping the agent"
            /sbin/stop amazon-ssm-agent
            sleep 1
        else
            echo "-> Agent is not running"
        fi
        # echo "Uninstalling the agent"
        dpkg -r amazon-ssm-agent
        sleep 1
    else
        echo "-> Agent is not installed in this instance"
    fi
elif [[ "$(cat /proc/1/comm)" == "systemd" ]]; then
	if [[ "$(systemctl is-active amazon-ssm-agent)" == "active" ]]; then
		# echo "-> Agent is running in the instance"
		# echo "Stopping the agent"
		systemctl stop amazon-ssm-agent
		# echo "Agent stopped"
		systemctl daemon-reload
		# echo "Reload daemon"
	else
		echo "-> Agent is not running in the instance"
	fi

	# echo "Uninstalling agent"
	dpkg -r amazon-ssm-agent

else
    echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms"
fi