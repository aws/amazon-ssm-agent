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
  pmOutput=$(rpm -U amazon-ssm-agent.rpm 2>&1)
  pmExit=$?
  echo "RPM Output: $pmOutput"

  if [ "$DO_REGISTER" = true ]; then
		/sbin/stop amazon-ssm-agent
		amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  agentVersion=$(rpm -q --qf '%{VERSION}\n' amazon-ssm-agent)
  echo "Installed version: '$agentVersion'"

  echo "starting agent"
  /sbin/start amazon-ssm-agent
  status amazon-ssm-agent

  if [ "$pmExit" -ne 0 ]; then
    if [[ $pmOutput == *"is already installed"* ]]; then
      echo "Install was successfull"
      exit 0
    fi

    echo "Package manager failed with exit code '$pmExit'"
    echo "Package manager output: $pmOutput"
    exit 125
  fi
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
  if [[ "$(systemctl is-active amazon-ssm-agent)" == "active" ]]; then
    echo "-> Agent is running in the instance"
    echo "Stopping the agent"
    systemctl stop amazon-ssm-agent
    echo "Agent stopped"
    systemctl daemon-reload
    echo "Reload daemon"
  else
		echo "-> Agent is not running in the instance"
  fi

  originalSvc=$(systemctl show -p FragmentPath amazon-ssm-agent.service)
  
  echo "Installing agent"
  pmOutput=$(rpm -U amazon-ssm-agent.rpm 2>&1)
  pmExit=$?
  echo "RPM Output: $pmOutput"

  updatedSvc=$(systemctl show -p FragmentPath amazon-ssm-agent.service)
  if ! [ -z "$originalSvc" ] && ! [ -z "$updatedSvc" ]; then
    if ! [ "$originalSvc" = "$updatedSvc" ]; then
      echo "Service file changed"
      originalSvc=${originalSvc#*=}
      updatedSvc=${updatedSvc#*=}
      SVC_SYMLINK="/etc/systemd/system/multi-user.target.wants/amazon-ssm-agent.service"
      if [ -f "$updatedSvc" ] && [ -L "$SVC_SYMLINK" ]; then
        if ! [ -e "$SVC_SYMLINK" ]; then
          echo "Found broken symlink"
          ln -nfs "$updatedSvc" "$SVC_SYMLINK"
        fi
      fi
    fi
  fi

  if [ "$DO_REGISTER" = true ]; then
    systemctl stop amazon-ssm-agent
    amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  agentVersion=$(rpm -q --qf '%{VERSION}\n' amazon-ssm-agent)
  echo "Installed version: '$agentVersion'"

  echo "Starting agent"
  systemctl daemon-reload
  systemctl start amazon-ssm-agent
  systemctl status amazon-ssm-agent

  if [ "$pmExit" -ne 0 ]; then
    if [[ $pmOutput == *"is already installed"* ]]; then
      echo "Install was successfull"
      exit 0
    fi

    echo "Package manager failed with exit code '$pmExit'"
    echo "Package manager output: $pmOutput"
    exit 125
  fi
else
  echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
  exit 124
fi
