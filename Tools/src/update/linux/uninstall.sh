#!/bin/bash

s3path=$1

echo "Uninstalling Amazon-ssm-agent"

# helper function to set error output
function error_exit
{
  echo "$1" 1>&2
  exit 1
}

PACKAGE_MANAGER='rpm'

# sets PACKAGE_MANAGER value to name of package manager
# that is passed in as long as it is present on the OS
function check_binary
{
    which $1 2>/dev/null
    RET_CODE=$?
    if [ ${RET_CODE} == 0 ];
    then
      PACKAGE_MANAGER=$1
      echo "Package manager found. Using ${PACKAGE_MANAGER}  to install amazon-ssm-agent."
    fi
}

check_binary yum
if [ ${PACKAGE_MANAGER} == "yum" ];
then
  UNINSTALL_COMMAND="yum -y remove amazon-ssm-agent"
else
  UNINSTALL_COMMAND="rpm --erase amazon-ssm-agent"
fi

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
  echo "Checking if the agent is installed"
  if [ "$(rpm -q amazon-ssm-agent)" != "package amazon-ssm-agent is not installed" ]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent using ${UNINSTALL_COMMAND}"
		${UNINSTALL_COMMAND}
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
  echo "Checking if the agent is installed"
  if [[ "$(systemctl status amazon-ssm-agent.service)" != *"Loaded: not-found"* ]]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent using ${UNINSTALL_COMMAND}"
		${UNINSTALL_COMMAND}
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
else
  echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
  exit 124
fi