#!/bin/bash

s3path=$1

echo "Uninstalling Amazon-ssm-agent"

# helper function to set error output
function error_exit
{
  echo "$1" 1>&2
  exit 1
}

function uninstall_agent()
{
  PACKAGE_MANAGER='rpm'
  which yum 2>/dev/null
  RET_CODE=$?
  if [ ${RET_CODE} == 0 ];
  then
    PACKAGE_MANAGER='yum'
    echo "Package manager found. Using ${PACKAGE_MANAGER}  to install amazon-ssm-agent."
  fi
  
  echo "Attempting to uninstall amazon-ssm-agent using yum"
  pmOutput=$(yum -y --cacheonly remove amazon-ssm-agent 2>&1)
  pmExit=$?
  echo "Yum Output: $pmOutput"
  if [ ${pmExit} -ne 0 ]; then
    echo "Yum uninstall failed. Attemting to uninstall amazon-ssm-agent using rpm"
    pmOutput=$(rpm --erase amazon-ssm-agent 2>&1)
    pmExit=$?
  fi
}

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
  echo "Checking if the agent is installed"
  if [ "$(rpm -q amazon-ssm-agent)" != "package amazon-ssm-agent is not installed" ]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent"
		uninstall_agent
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
  echo "Checking if the agent is installed"
  if [[ "$(systemctl status amazon-ssm-agent.service)" != *"Loaded: not-found"* ]]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent"
		uninstall_agent
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
else
  echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
  exit 124
fi