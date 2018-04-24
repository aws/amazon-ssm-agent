#!/bin/bash
 
echo "Installing snap package"
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

if [[ "$(cat /proc/1/comm)" == "systemd" ]]; then
 
	if [[ "$(systemctl is-active snap.amazon-ssm-agent.amazon-ssm-agent)" == "active" ]]; then
	    echo "detected snap amazon-ssm-agent running on the system, installing new snaps..."
        # stop the current agent 
        systemctl stop snap.amazon-ssm-agent.amazon-ssm-agent.service
        echo 'SSM Agent uninstalled'
    else 
        echo "-> Agent is not running in the instance "

    fi
    if [[ ! -f amazon-ssm-agent.snap || ! -f amazon-ssm-agent.assert ]]; then
        echo '[ERROR] Snap is not available for this version. Please uninstall the snap and install a debian if this agent version is required.'
        exit 1
    else
        # acknowledge the signature pulled from the s3 distro
        snap ack amazon-ssm-agent.assert
        # install snap in classic mode
        echo 'installing snap'
        snap install --classic amazon-ssm-agent.snap
        # register onprem instance
        if [ "$DO_REGISTER" = true ]; then
            snap stop amazon-ssm-agent
            SNAP_BIN=/snap/amazon-ssm-agent/current
            "$SNAP_BIN"/amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
        fi
        
        echo "Starting agent..."
        snap start amazon-ssm-agent
    fi
else 
    error_exit '[ERROR] Snap install is not supported on this instance. Please uninstall the snap agent and try again'
fi
        