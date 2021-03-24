// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// +build freebsd linux netbsd openbsd darwin

// Package domainjoin implements the domainjoin plugin.
package domainjoin

func getDomainJoinScript() []string {
	var scriptContents = awsDomainJoinScript

	return []string{scriptContents}
}

const awsDomainJoinScript = `#!/bin/sh

# Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may not
# use this file except in compliance with the License. A copy of the
# License is located at
#
# http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
# either express or implied. See the License for the specific language governing
# permissions and limitations under the License.

DIRECTORY_ID=""
DIRECTORY_NAME=""
DIRECTORY_OU=""
REALM=""
DNS_IP_ADDRESS1=""
DNS_IP_ADDRESS2=""
LINUX_DISTRO=""
CURTIME=""
REGION=""
# https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-linux.html
AWSCLI="/usr/local/bin/aws"
# Service Creds from Secrets Manager
DOMAIN_USERNAME=""
DOMAIN_PASSWORD=""
# Secrets Manager Secret ID needs to be of the form aws/directory-services/d-91673491b6/seamless-domain-join
SECRET_ID_PREFIX="aws/directory-services"
KEEP_HOSTNAME=""
AWS_CLI_INSTALL_DIR="$PWD/"

##################################################
## Set hostname to NETBIOS computer name #########
##################################################
set_hostname() {
    INSTANCE_NAME=$(hostname --short) 2>/dev/null

    # NetBIOS computer names consist of up to 15 bytes of OEM characters
    # https://docs.microsoft.com/en-us/windows/win32/sysinfo/computer-names?redirectedfrom=MSDN

    # Naming conventions in Active Directory
    # https://support.microsoft.com/en-us/help/909264/naming-conventions-in-active-directory-for-computers-domains-sites-and
    RANDOM_COMPUTER_NAME=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1)
    COMPUTER_NAME=$(echo EC2AMAZ-$RANDOM_COMPUTER_NAME)
    HOSTNAMECTL=$(which hostnamectl)
    if [ ! -z "$HOSTNAMECTL" ]; then
        hostnamectl set-hostname $COMPUTER_NAME.$DIRECTORY_NAME >/dev/null
    else
        hostname $COMPUTER_NAME.$DIRECTORY_NAME >/dev/null
    fi
    if [ $? -ne 0 ]; then echo "***Failed: set_hostname(): set hostname failed" && exit 1; fi

    # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/set-hostname.html
    if [ -f /etc/sysconfig/network ]; then
            sed -i "s/HOSTNAME=.*$//g" /etc/sysconfig/network
        echo "HOSTNAME=$COMPUTER_NAME.$DIRECTORY_NAME" >> /etc/sysconfig/network
    fi
}

##################################################
## Download AWS CLI zip file #####################
##################################################
download_awscli_zipfile() {
    MAX_RETRIES=3
    CURL_DOWNLOAD_URL="$1"
    if [ -z "$1" ]; then
       echo "***Failed: No URL argument provided for curl"
       exit 1
    fi
    echo "CURL download url is $CURL_DOWNLOAD_URL"

    MAX_RETRIES=3
    STATUS=1
    for i in $(seq 1 $MAX_RETRIES)
    do
       echo "[$i] Attempt installing AWS CLI by using curl"
       curl --fail $CURL_DOWNLOAD_URL -o "$AWS_CLI_INSTALL_DIR/awscliv2.zip"
       STATUS=$?
       if [ $STATUS -eq 0 ]; then
          break
       else
           curl -1 --fail $CURL_DOWNLOAD_URL -o "$AWS_CLI_INSTALL_DIR/awscliv2.zip"
           STATUS=$?
           if [ $STATUS -eq 0 ]; then
              break
           fi
       fi
       sleep 3
    done

    if [ $STATUS -ne 0 ]; then
       echo "***Failed: curl $CURL_DOWNLOAD_URL failed."
       exit 1
    fi
}

###################################################
## Check permissions of AWS CLI install directory #
###################################################
check_awscli_install_dir() {
   if [ ! -d $AWS_CLI_INSTALL_DIR ]; then
       echo "***Failed: AWS CLI install dir $AWS_CLI_INSTALL_DIR does not exist"
       exit 1
   fi

   # POSIX ls specification - https://pubs.opengroup.org/onlinepubs/9699919799/
   # If the -l option is specified, the following information
   # shall be written for files other than character special
   # and block special files: "%s %u %s %s %u %s %s\n", <file mode>, <number of links>, <owner name>, <group name>, <size>, <date and time>, <pathname>
   ls -ld $AWS_CLI_INSTALL_DIR
   if [ $? -ne 0 ]; then
       echo "***Failed: ls -ld $AWS_CLI_INSTALL_DIR"
       exit 1
   fi
   AWS_CLI_INSTALL_DIR_LS_LD=$(ls -ld $AWS_CLI_INSTALL_DIR)
   AWS_CLI_INSTALL_DIR_OWNER=$(echo $AWS_CLI_INSTALL_DIR_LS_LD | awk '{print $3}')
   id -u $AWS_CLI_INSTALL_DIR_OWNER
   if [ $? -ne 0 ]; then
       echo "***Failed: id command"
       exit 1
   fi
   ID=$(id -u $AWS_CLI_INSTALL_DIR_OWNER)
   if [ $ID != "0" ]; then
       echo "***Failed: AWS CLI install dir user is not root"
       exit 1
   fi
   AWS_CLI_INSTALL_DIR_PERMISSIONS=$(echo $AWS_CLI_INSTALL_DIR_LS_LD | awk '{print $1}' | cut -c 5-)
   if echo "$AWS_CLI_INSTALL_DIR_PERMISSIONS" | grep -e "------" ; then
       echo "Permissions check successful for AWS CLI install directory"
   else
       echo "***Failed: Wrong permissions for $AWS_CLI_INSTALL_DIR_PERMISSIONS"
       exit 1
   fi
}

##################################################
########## Install components ####################
##################################################
install_components() {
    LINUX_DISTRO=$(cat /etc/os-release | grep NAME | awk -F'=' '{print $2}')
    LINUX_DISTRO_VERSION_ID=$(cat /etc/os-release | grep VERSION_ID | awk -F'=' '{print $2}' | tr -d '"')
    if [ -z $LINUX_DISTRO_VERSION_ID ]; then
       echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
       exit 1
    fi

    if grep 'CentOS' /etc/os-release 1>/dev/null 2>/dev/null; then
        if [ "$LINUX_DISTRO_VERSION_ID" -lt "7" ] ; then
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
        fi
        LINUX_DISTRO='CentOS'
        # yum -y update
        ## yum update takes too long
        yum -y install realmd adcli oddjob-mkhomedir oddjob samba-winbind-clients samba-winbind samba-common-tools samba-winbind-krb5-locator krb5-workstation unzip bind-utils >/dev/null
        if [ $? -ne 0 ]; then echo "install_components(): yum install errors for CentOS" && return 1; fi
    elif grep -e 'Red Hat' /etc/os-release 1>/dev/null 2>/dev/null; then
        LINUX_DISTRO='RHEL'
        RHEL_MAJOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $1}')
        RHEL_MINOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $2}')
        if [ $RHEL_MAJOR_VERSION -eq "7" ] && [ ! -z $RHEL_MINOR_VERSION ] && [ $RHEL_MINOR_VERSION -lt "6" ]; then
            # RHEL 7.5 and below are not supported
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
        fi
        if [ $RHEL_MAJOR_VERSION -eq "7" ] && [ -z $RHEL_MINOR_VERSION ]; then
            # RHEL 7 is not supported
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
        fi
        # yum -y update
        ## yum update takes too long
        # https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html-single/deploying_different_types_of_servers/index
        yum -y  install realmd adcli oddjob-mkhomedir oddjob samba-winbind-clients samba-winbind samba-common-tools samba-winbind-krb5-locator krb5-workstation python3 vim unzip bind-utils >/dev/null
        alias python=python3
        if [ $? -ne 0 ]; then echo "install_components(): yum install errors for Red Hat" && return 1; fi
    elif grep -e 'Fedora' /etc/os-release 1>/dev/null 2>/dev/null; then
        LINUX_DISTRO='Fedora'
        ## yum update takes too long, but it is unavoidable here.
        yum -y update
        yum -y  install realmd adcli oddjob-mkhomedir oddjob samba-winbind-clients samba-winbind samba-common-tools samba-winbind-krb5-locator krb5-workstation python3 vim unzip bind-utils >/dev/null
        alias python=python3
        if [ $? -ne 0 ]; then echo "install_components(): yum install errors for Fedora" && return 1; fi
        systemctl restart dbus
    elif grep 'Amazon Linux' /etc/os-release 1>/dev/null 2>/dev/null; then
         LINUX_DISTRO='AMAZON_LINUX'
         # yum -y update
         ## yum update takes too long
         yum -y  install realmd adcli oddjob-mkhomedir oddjob samba-winbind-clients samba-winbind samba-common-tools samba-winbind-krb5-locator krb5-workstation unzip bind-utils >/dev/null
         if [ $? -ne 0 ]; then echo "install_components(): yum install errors for Amazon Linux" && return 1; fi
    elif grep 'Ubuntu' /etc/os-release 1>/dev/null 2>/dev/null; then
         LINUX_DISTRO='UBUNTU'
         UBUNTU_MAJOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $1}')
         UBUNTU_MINOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $2}')
         if [ $UBUNTU_MAJOR_VERSION -lt "14" ]; then
            # Ubuntu versions below 14.04 are not supported
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
         fi
         # set DEBIAN_FRONTEND variable to noninteractive to skip any interactive post-install configuration steps.
         export DEBIAN_FRONTEND=noninteractive
         apt-get -y update
         if [ $? -ne 0 ]; then echo "install_components(): apt-get update errors for Ubuntu" && return 1; fi
         apt-get -yq install realmd adcli winbind samba libnss-winbind libpam-winbind libpam-krb5 krb5-config krb5-locales krb5-user packagekit  ntp unzip python dnsutils > /dev/null
         if [ $? -ne 0 ]; then echo "install_components(): apt-get install errors for Ubuntu" && return 1; fi
         # Disable Reverse DNS resolution. Ubuntu Instances must be reverse-resolvable in DNS before the realm will work.
         sed -i "s/default_realm.*$/default_realm = $REALM\n\trdns = false/g" /etc/krb5.conf
         if [ $? -ne 0 ]; then echo "install_components(): access errors to /etc/krb5.conf"; return 1; fi
         if ! grep "Ubuntu 16.04" /etc/os-release 2>/dev/null; then
             pam-auth-update --enable mkhomedir
         fi
    elif grep 'SUSE Linux' /etc/os-release 1>/dev/null 2>/dev/null; then
         SUSE_MAJOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $1}')
         SUSE_MINOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $2}')
         if [ "$SUSE_MAJOR_VERSION" -lt "15" ]; then
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
         fi
         if [ "$SUSE_MAJOR_VERSION" -eq "15" ]; then
            sudo SUSEConnect -p PackageHub/15.1/x86_64
         fi
         LINUX_DISTRO='SUSE'
         sudo zypper update -y
         sudo zypper -n install realmd adcli sssd sssd-tools sssd-ad samba-client krb5-client samba-winbind krb5-client python bind-utils
         if [ $? -ne 0 ]; then
            return 1
         fi
         alias python=python3
    elif grep 'Debian' /etc/os-release; then
         DEBIAN_MAJOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $1}')
         DEBIAN_MINOR_VERSION=$(echo $LINUX_DISTRO_VERSION_ID | awk -F'.' '{print $2}')
         if [ "$DEBIAN_MAJOR_VERSION" -lt "9" ]; then
            echo "**Failed : Unsupported OS version $LINUX_DISTRO : $LINUX_DISTRO_VERSION_ID"
            exit 1
         fi
         apt-get -y update
         LINUX_DISTRO='DEBIAN'
         DEBIAN_FRONTEND=noninteractive apt-get -yq install realmd adcli winbind samba libnss-winbind libpam-winbind libpam-krb5 krb5-config krb5-locales krb5-user packagekit  ntp unzip dnsutils > /dev/null
         if [ $? -ne 0 ]; then
            return 1
         fi
    fi

    check_awscli_install_dir
    if uname -a | grep -e "x86_64" -e "amd64"; then
        download_awscli_zipfile "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip"
    elif uname -a | grep "aarch64"; then
        download_awscli_zipfile "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip"
    else
        echo "***Failed: install_components processor type is unsupported." && exit 1
    fi

    unzip -o "$AWS_CLI_INSTALL_DIR"/awscliv2.zip 1>/dev/null
    if [ $? -ne 0 ]; then echo "***Failed: unzip of awscliv2.zip" && exit 1; fi
    "$AWS_CLI_INSTALL_DIR"/aws/install -u 1>/dev/null
    if [ $? -ne 0 ]; then echo "***Failed: aws cli install" && exit 1; fi
    return 0
}

######################################################################
##### Retrieve Service Account Credentials from Secrets Manager   ####
######################################################################
get_servicecreds() {
    SECRET_ID="${SECRET_ID_PREFIX}/$DIRECTORY_ID/seamless-domain-join"
    secret=$(/usr/local/bin/aws secretsmanager get-secret-value --secret-id "$SECRET_ID" --region $REGION 2>/dev/null)
    DOMAIN_USERNAME=$(echo $secret |  python -c 'import sys, json; obj=json.load(sys.stdin); print(obj["SecretString"])' | python -c 'import json,sys; obj=json.load(sys.stdin); print(obj["awsSeamlessDomainUsername"])')
    if [ $? -ne 0 ]; then
        echo "***Failed: Cannot find awsSeamlessDomainUsername in $SECRET_ID in Secrets Manager"
        exit 1
    fi
    DOMAIN_PASSWORD=$(echo $secret |  python -c 'import sys, json; obj=json.load(sys.stdin); print(obj["SecretString"])' | python -c 'import json,sys; obj=json.load(sys.stdin); print(obj["awsSeamlessDomainPassword"])')
    if [ $? -ne 0 ]; then
        echo "***Failed: aws secretsmanager get-secret-value --secret-id $SECRET_ID --region $REGION"
        exit 1
    fi
}

##################################################
## Setup resolv.conf and also dhclient.conf ######
## to prevent overwriting of resolv.conf    ######
##################################################
setup_resolv_conf_and_dhclient_conf() {
    if [ ! -z "$DNS_IP_ADDRESS1" ] && [ ! -z "$DNS_IP_ADDRESS2" ]; then
        touch /etc/resolv.conf
        mv /etc/resolv.conf /etc/resolv.conf.backup."$CURTIME"
        echo ";Generated by Domain Join SSMDocument" > /etc/resolv.conf
        echo "search $DIRECTORY_NAME" >> /etc/resolv.conf
        echo "nameserver $DNS_IP_ADDRESS1" >> /etc/resolv.conf
        echo "nameserver $DNS_IP_ADDRESS2" >> /etc/resolv.conf
        touch /etc/dhcp/dhclient.conf
        mv /etc/dhcp/dhclient.conf /etc/dhcp/dhclient.conf.backup."$CURTIME"
        echo "supersede domain-name-servers $DNS_IP_ADDRESS1, $DNS_IP_ADDRESS2;" > /etc/dhcp/dhclient.conf
    elif [ ! -z "$DNS_IP_ADDRESS1" ] && [ -z "$DNS_IP_ADDRESS2" ]; then
        touch /etc/resolv.conf
        mv /etc/resolv.conf /etc/resolv.conf.backup."$CURTIME"
        echo ";Generated by Domain Join SSMDocument" > /etc/resolv.conf
        echo "search $DIRECTORY_NAME" >> /etc/resolv.conf
        echo "nameserver $DNS_IP_ADDRESS1" >> /etc/resolv.conf
        touch /etc/dhcp/dhclient.conf
        mv /etc/dhcp/dhclient.conf /etc/dhcp/dhclient.conf.backup."$CURTIME"
        echo "supersede domain-name-servers $DNS_IP_ADDRESS1;" > /etc/dhcp/dhclient.conf
    elif [ -z "$DNS_IP_ADDRESS1" ] && [ ! -z "$DNS_IP_ADDRESS2" ]; then
        touch /etc/resolv.conf
        mv /etc/resolv.conf /etc/resolv.conf.backup."$CURTIME"
        echo ";Generated by Domain Join SSMDocument" > /etc/resolv.conf
        echo "search $DIRECTORY_NAME" >> /etc/resolv.conf
        echo "nameserver $DNS_IP_ADDRESS2" >> /etc/resolv.conf
        touch /etc/dhcp/dhclient.conf
        mv /etc/dhcp/dhclient.conf /etc/dhcp/dhclient.conf.backup."$CURTIME"
        echo "supersede domain-name-servers $DNS_IP_ADDRESS2;" > /etc/dhcp/dhclient.conf
    else
        echo "***Failed: No DNS IPs available" && exit 1
    fi
}

##################################################
## Set PEER_DNS to yes ###########################
##################################################
set_peer_dns() {
    for f in $(ls /etc/sysconfig/network-scripts/ifcfg-*)
    do
        if echo $f | grep "lo"; then
            continue
        fi
        if ! grep PEERDNS $f; then
            echo "" >> $f
            echo PEERDNS=yes >> $f
        fi
    done
}

##################################################
## Print shell variables #########################
##################################################
print_vars() {
    echo "REGION = $REGION"
    echo "DIRECTORY_ID = $DIRECTORY_ID"
    echo "DIRECTORY_NAME = $DIRECTORY_NAME"
    echo "DIRECTORY_OU = $DIRECTORY_OU"
    echo "REALM = $REALM"
    echo "DNS_IP_ADDRESS1 = $DNS_IP_ADDRESS1"
    echo "DNS_IP_ADDRESS2 = $DNS_IP_ADDRESS2"
    echo "COMPUTER_NAME = $COMPUTER_NAME"
    echo "hostname = $(hostname)"
    echo "LINUX_DISTRO = $LINUX_DISTRO"
}

#########################################################
## Add FQDN and Hostname to Hosts file for below error ##
# No DNS domain configured for ip-172-31-12-23.         #
# Unable to perform DNS Update.                         #
#########################################################
configure_hosts_file() {
    fullhost="${COMPUTER_NAME}.${DIRECTORY_NAME}"  # ,, means lowercase since bash v4
    ip_address="$(ip -o -4 addr show eth0 | awk '{print $4}' | cut -d/ -f1)"
    cleanup_comment='# Generated by Domain Join SSMDocument'
    sed -i".orig" -r\
        "/^.*${cleanup_comment}/d;\
        /^127.0.0.1\s+localhost\s*/a\\${ip_address} ${fullhost} ${COMPUTER_NAME} ${cleanup_comment}" /etc/hosts
}

##################################################
## Add AWS Directory Service DNS IP Addresses as #
## primary to the resolv.conf and dhclient       #
## configuration files.                          #
##################################################
do_dns_config() {
    setup_resolv_conf_and_dhclient_conf
    if [ $LINUX_DISTRO = 'AMAZON_LINUX' ]; then
        set_peer_dns
    fi

    if [ $LINUX_DISTRO = "UBUNTU" ]; then
        if [ -d /etc/netplan ]; then
            # Ubuntu 18.04
            cat << EOF | tee /etc/netplan/99-custom-dns.yaml
network:
    version: 2
    ethernets:
        eth0:
            nameservers:
                addresses: [$DNS_IP_ADDRESS1, $DNS_IP_ADDRESS2]
            dhcp4-overrides:
                use-dns: false
EOF
            netplan apply
            if [ $? -ne 0 ]; then echo "***Failed: do_dns_config(): netplan apply failed" && exit 1; fi
            # Seems to fail otherwise
            sleep 15
        fi
    fi

    if [ $LINUX_DISTRO = "RHEL" ] || [ $LINUX_DISTRO = "Fedora" ]; then
        set_peer_dns
        if [ -f /etc/NetworkManager/NetworkManager.conf ]; then
            cp /etc/NetworkManager/NetworkManager.conf /etc/NetworkManager/NetworkManager.conf."$CURTIME"
            cat /etc/NetworkManager/NetworkManager.conf."$CURTIME" | sed "s/\[main\]/[main]\ndns=none/g" > /etc/NetworkManager/NetworkManager.conf
        fi
    fi

    if [ $LINUX_DISTRO = "CentOS" ]; then
        set_peer_dns
    fi
}

##################################################
## DNS IP reachability test to                  ##
## catch invalid or unreachable DNS IPs         ##
##################################################
is_dns_ip_reachable() {
    DNS_IP="$1"
    ping -c 1 "$DNS_IP" 2>/dev/null
    if [ $? -eq 0 ]; then
            return 0
    fi

    return 1
}

##################################################
## Resolve domain name to IP address(es)        ##
## by using nslookup command                    ##
##################################################
resolve_name_to_ip() {
    (
        nslookup "$1"| tail -n +3 | sed -n 's/Address:\s*//p'
    ) && return 0 || return 1
}

##################################################
## DNS may already be reachable if DHCP option  ##
## sets are used.                               ##
##################################################
is_directory_reachable() {
    DNS_IPS=$(resolve_name_to_ip $DIRECTORY_NAME)
    if [ $? -ne 0 ]; then
        echo "***Failed: Cannot resolve domain name $DIRECTORY_NAME" && return 1
    fi
    echo -e "Successfully resolve domain name $DIRECTORY_NAME to IP address(es):\n$DNS_IPS"

    DNS_IP1=$(echo $DNS_IPS | awk '{ print $1 }')
    DNS_IP2=$(echo $DNS_IPS | awk '{ print $2 }')

    MAX_RETRIES=5
    for i in $(seq 1 $MAX_RETRIES)
    do
        is_dns_ip_reachable $DNS_IP1
        if [ $? -eq 0 ]; then
            return 0
        fi

        is_dns_ip_reachable $DNS_IP2
        if [ $? -eq 0 ]; then
            return 0
        fi
    done

    return 1
}

##################################################
## Join Linux instance to AWS Directory Service ##
##################################################
do_domainjoin() {
    MAX_RETRIES=10
    for i in $(seq 1 $MAX_RETRIES)
    do
        if [ -z "$DIRECTORY_OU" ]; then
            LOG_MSG=$(echo $DOMAIN_PASSWORD | realm join --client-software=winbind -U ${DOMAIN_USERNAME}@${DIRECTORY_NAME} "$DIRECTORY_NAME" -v 2>&1)
        else
            LOG_MSG=$(echo $DOMAIN_PASSWORD | realm join --client-software=winbind -U ${DOMAIN_USERNAME}@${DIRECTORY_NAME} "$DIRECTORY_NAME" --computer-ou="$DIRECTORY_OU" -v 2>&1)
        fi
        STATUS=$?
        if [ $STATUS -eq 0 ]; then
            break
        else
            if echo "$LOG_MSG" | grep -q "Already joined to this domain"; then
                echo "do_domainjoin(): Already joined to this domain : $LOG_MSG"
                STATUS=0
                break
            fi
        fi
        sleep 10
    done

    if [ $STATUS -ne 0 ]; then
        echo "***Failed: realm join failed" && exit 1
    fi
    echo "########## SUCCESS: realm join successful $LOG_MSG ##########"
}

##############################
## Configure nsswitch.conf  ##
##############################
config_nsswitch() {
    # Edit nsswitch config
    NSSWITCH_CONF_FILE=/etc/nsswitch.conf
    sed -i 's/^\s*passwd:.*$/passwd:     compat winbind/' $NSSWITCH_CONF_FILE
    sed -i 's/^\s*group:.*$/group:      compat winbind/' $NSSWITCH_CONF_FILE
    sed -i 's/^\s*shadow:.*$/shadow:     compat winbind/' $NSSWITCH_CONF_FILE
}

###################################################
## Configure id-mappings in Samba                ##
###################################################
config_samba() {
    AD_INFO=$(adcli info ${DIRECTORY_NAME} | grep '^domain-short = ' | awk '{print $3}')
    sed -i".pre-join" -r\
        "/^\[global\]/a\\
        idmap config * : backend = autorid\n\
        idmap config * : range = 100000000-2100000000\n\
        idmap config * : rangesize = 100000000\n\
        idmap config ${AD_INFO} : backend = rid\n\
        idmap config ${AD_INFO} : range = 65536 - 99999999\n\
        winbind refresh tickets = yes\n\
        kerberos method = secrets and keytab\n\
        winbind enum groups = no\n\
        winbind enum users = no
        /^\s*idmap/d;\
        /^\s*kerberos\s+method/d;\
        /^\s*winbind\s+refresh/d;\
        /^\s*winbind\s+enum/d"\
        /etc/samba/smb.conf

    cp /etc/samba/smb.conf /tmp

    # Flushing Samba Winbind databases
    net cache flush

    # Restarting Winbind daemon
    service winbind restart
}

reconfigure_samba() {
    sed -i 's/kerberos method = system keytab/kerberos method = secrets and keytab/g' /etc/samba/smb.conf
    service winbind restart
    if [ $? -ne 0 ]; then
        systemctl restart winbind
        if [ $? -ne 0 ]; then
            service winbind restart
        fi
    fi 
}

##################################################
## Main entry point ##############################
##################################################
CURTIME=$(date | sed 's/ //g')

#if [ $# -eq 0 ]; then
#    exit 1
#fi

for i in "$@"; do
    case "$i" in
        --directory-id)
            shift
            DIRECTORY_ID="$1"
            continue
            ;;
        --directory-name)
            shift;
            DIRECTORY_NAME="$1"
            continue
            ;;
        --directory-ou)
            shift;
            DIRECTORY_OU="$1"
            continue
            ;;
        --instance-region)
            shift;
            REGION="$1"
            continue
            ;;
        --dns-addresses)
            shift;
            DNS_ADDRESSES="$1"
            DNS_IP_ADDRESS1=$(echo $DNS_ADDRESSES | awk -F',' '{ print $1 }')
            DNS_IP_ADDRESS2=$(echo $DNS_ADDRESSES | awk -F',' '{ print $2 }')
            if [ ! -z $DNS_IP_ADDRESS1 ]; then
                is_dns_ip_reachable $DNS_IP_ADDRESS1
                if [ $? -ne 0 ]; then
                    echo "**Failed: Unable to reach DNS server $DNS_IP_ADDRESS1"
                    exit 1
                fi
            fi
            if [ ! -z $DNS_IP_ADDRESS2 ]; then
                is_dns_ip_reachable $DNS_IP_ADDRESS2
                if [ $? -ne 0 ]; then
                    echo "**Failed: Unable to reach DNS server $DNS_IP_ADDRESS2"
                    exit 1
                fi
            fi
            continue
            ;;
        --proxy-address)
            shift;
            PROXY_ADDRESS="$1"
            continue
            ;;
        --no-proxy)
            shift;
            NO_PROXY="$1"
            continue
            ;;
        --keep-hostname)
            shift;
            KEEP_HOSTNAME="TRUE"
            continue
            ;;
    esac
    shift
done

if [ -z $REGION ]; then
    echo "***Failed: No Region found" && exit 1
fi

# Deal with scenario where this script is run again after the domain is already joined.
# We want to avoid rerunning as the set_hostname function can change the hostname of a server that is already
# domain joined and cause a mismatch. 
realm list | grep -q "domain-name: ${DIRECTORY_NAME}\$"
if [ $? -eq 0 ]; then
    echo "########## SKIPPING Domain Join: ${DIRECTORY_NAME} already joined  ##########"
    exit 0
fi

REALM=$(echo "$DIRECTORY_NAME" | tr [a-z] [A-Z])

COMPUTER_NAME=$(hostname --short)
if [ -z $KEEP_HOSTNAME ]; then
   set_hostname
fi
configure_hosts_file

MAX_RETRIES=8
for i in $(seq 1 $MAX_RETRIES)
do
    echo "[$i] Attempt installing components"
    install_components
    if [ $? -eq 0 ]; then
        break
    fi
    sleep 30
done

if [ -z $DNS_IP_ADDRESS1 ] && [ -z $DNS_IP_ADDRESS2 ]; then
    DNS_ADDRESSES=$($AWSCLI ds describe-directories --region $REGION --directory-id $DIRECTORY_ID --output text | grep DNSIPADDR | awk '{print $2}')
    if [ $? -ne 0 ]; then
        echo "***Failed: DNS IPs not found" && exit 1
    fi
    DNS_IP_ADDRESS1=$(echo $DNS_ADDRESSES | awk '{ print $1 }')
    DNS_IP_ADDRESS2=$(echo $DNS_ADDRESSES | awk '{ print $2 }')
fi

## Configure DNS even if DHCP option set is used.
do_dns_config
sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/g' /etc/ssh/sshd_config
systemctl restart sshd
if [ $? -ne 0 ]; then
   systemctl restart ssh
   if [ $? -ne 0 ]; then
      service sshd restart
   fi
   if [ $? -ne 0 ]; then
      service ssh restart
   fi
fi

print_vars
is_directory_reachable
if [ $? -eq 0 ]; then
    config_nsswitch
    get_servicecreds
    config_samba
    do_domainjoin
    reconfigure_samba
else
    echo "**Failed: Unable to reach DNS server"
    exit 1
fi

echo "Success"
exit 0`
