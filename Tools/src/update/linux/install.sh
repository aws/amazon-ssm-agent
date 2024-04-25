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

# allow ssm-agent to finish its work
sleep 2

function install_agent
{
  PACKAGE_MANAGER='rpm'
  which yum 2>/dev/null
  RET_CODE=$?
  if [ ${RET_CODE} == 0 ];
  then
    PACKAGE_MANAGER='yum'
    echo "Package manager found. Using ${PACKAGE_MANAGER}  to install amazon-ssm-agent."
  fi
  
  echo "Attempting to install amazon-ssm-agent using yum"
  pmOutput=$(yum -y localinstall amazon-ssm-agent.rpm 2>&1)
  pmExit=$?
  echo "Yum Output: $pmOutput"
  if [ ${pmExit} -ne 0 ]; then
    echo "Yum install failed. Attemting install amazon-ssm-agent using rpm"
    pmOutput=$(rpm -U amazon-ssm-agent.rpm 2>&1)
    pmExit=$?
  fi

  if [ "$pmExit" -ne 0 ]; then
    # messages returned by rpm / yum when trying to install package that is already installed
    if [[ $pmOutput == *"is already installed"* ]] || [[ $pmOutput == *"does not update installed package"* ]]; then
      echo "Install was successful"
      exit 0
    fi

    echo "Package manager failed with exit code '$pmExit'"
    echo "Package manager output: $pmOutput"
    exit 125
  fi
}

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
  echo "upstart detected"
  install_agent

  if [ "$DO_REGISTER" = true ]; then
		/sbin/stop amazon-ssm-agent
		amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  agentVersion=$(rpm -q --qf '%{VERSION}\n' amazon-ssm-agent)
  echo "Installed version: '$agentVersion'"

  echo "starting agent"
  /sbin/start amazon-ssm-agent
  status amazon-ssm-agent
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
  if [[ "$(systemctl is-active amazon-ssm-agent.service)" == "active" ]]; then
    echo "-> Agent is running in the instance"
    echo "Stopping the agent"
    systemctl stop amazon-ssm-agent.service
    echo "Agent stopped"
    systemctl daemon-reload
    echo "Reload daemon"
  else
		echo "-> Agent is not running in the instance"
  fi

  originalSvc=$(systemctl show -p FragmentPath amazon-ssm-agent.service)
  
  install_agent

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
    systemctl stop amazon-ssm-agent.service
    amazon-ssm-agent -register -code "$RMI_CODE" -id "$RMI_ID" -region "$RMI_REGION"
  fi

  agentVersion=$(rpm -q --qf '%{VERSION}\n' amazon-ssm-agent)
  echo "Installed version: '$agentVersion'"

  echo "Starting agent"
  systemctl daemon-reload
  systemctl start amazon-ssm-agent.service
  systemctl status amazon-ssm-agent.service
else
  echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
  exit 124
fi
