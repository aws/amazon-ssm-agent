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

launchctl list com.amazon.aws.ssm >/dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "-> Agent is running in the instance"
    echo "Stopping the agent"
    launchctl unload -w /Library/LaunchDaemons/com.amazon.aws.ssm.plist
    echo "Agent stopped"
else
    echo "-> Agent is not running in the instance"
fi

echo "Installing agent"
sudo tar -xvf amazon-ssm-agent.tar.gz -C /

if [ "$DO_REGISTER" = true ]; then
    /opt/aws/ssm/amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
fi

echo "Starting agent"
launchctl load -w /Library/LaunchDaemons/com.amazon.aws.ssm.plist
launchctl start com.amazon.aws.ssm
echo "$(launchctl list com.amazon.aws.ssm)"


