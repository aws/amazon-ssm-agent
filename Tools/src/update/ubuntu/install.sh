#!/bin/bash

echo "Installing ubuntu pkg"
# helper function to set error output
function error_exit
{
  echo "$1" 1>&2
  exit 1
}

function get_installed_agent_version {
  dpkg-query -f='${Status}\t${version}\n' -W amazon-ssm-agent | grep "^install ok installed" | awk -F'\t' '{print $2}'
}

# helper function to check if agent is installed
function is_agent_installed {
  INSTALLED_AGENT_VERSION=`get_installed_agent_version`
  DEB_AGENT_VERSION=`dpkg -I amazon-ssm-agent.deb | grep '^ Version: ' | awk '{print $2}'`

  if [ -z "$INSTALLED_AGENT_VERSION" ]
  then
    echo "Failed to get installed agent version"
    # 1 == false
    return 1
  fi

  if [ -z "$DEB_AGENT_VERSION" ]
  then
    echo "Failed to get deb agent version"
    # 1 == false
    return 1
  fi

  if [ "$INSTALLED_AGENT_VERSION" == "$DEB_AGENT_VERSION" ]
  then
    echo "Correct agent version '$INSTALLED_AGENT_VERSION' is installed"
    # 0 == true
    return 0
  else
    echo "Incorrect agent version is installed"
    echo "  Installed agent version: $INSTALLED_AGENT_VERSION"
    echo "  Requested agent version: $DEB_AGENT_VERSION"
    # 1 == false
    return 1
  fi
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

if [[ "$(cat /proc/1/comm)" == "init" ]]; then
  if [ "$(dpkg -s amazon-ssm-agent | grep 'Status:')" == "Status: install ok installed" ]; then
    if [ "$(status amazon-ssm-agent)" != "amazon-ssm-agent stop/waiting" ]; then
      echo "-> Agent is running in the instance"
      echo "Stopping the agent"
      /sbin/stop amazon-ssm-agent
      sleep 1
    fi
  fi

  echo "Installing agent"
  dpkg -i amazon-ssm-agent.deb
  pmExit=$?

  agentVersion=$(get_installed_agent_version)
  echo "Installed version: '$agentVersion'"

  if [ "$DO_REGISTER" = true ]; then
    /sbin/stop amazon-ssm-agent
    amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  echo "Starting agent"
  /sbin/start amazon-ssm-agent

  echo "Status"
  status amazon-ssm-agent

  if [ "$pmExit" -ne 0 ]; then
    echo "Package manager failed with exit code '$pmExit'"

    if is_agent_installed; then
      echo "The agent installed successfully"
      exit 0
    fi

    exit 125
  fi

elif [[ "$(cat /proc/1/comm)" == "systemd" ]]; then
  if [[ "$(systemctl is-active amazon-ssm-agent.service)" == "active" ]]; then
    echo "-> Agent is running in the instance"
    systemctl stop amazon-ssm-agent.service
    echo "Agent stopped"
    systemctl daemon-reload
    echo "Reload daemon"
  else
    echo "-> Agent is not running on the instance."
  fi

  echo "Installing agent"
  dpkg -i amazon-ssm-agent.deb
  pmExit=$?

  if [ "$DO_REGISTER" = true ]; then
    systemctl stop amazon-ssm-agent.service
    amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  agentVersion=$(get_installed_agent_version)
  echo "Installed version: '$agentVersion'"

  echo "Starting agent"
  systemctl daemon-reload
  systemctl start amazon-ssm-agent.service

  echo "Status"
  systemctl status amazon-ssm-agent.service

  if [ "$pmExit" -ne 0 ]; then
    echo "Package manager failed with exit code '$pmExit'"

    if is_agent_installed; then
      echo "The agent installed successfully"
      exit 0
    fi

    exit 125
  fi

else
    echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
    exit 124
fi