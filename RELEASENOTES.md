Latest
===============
- Updated snapcraft.yml specification

3.3.808.0
===============
- Add enhancements related to KMS sessions
- Add support for RHEL 8.10 & 9.4
- Allow in-place upgrade for hybrid distributor packages
- Fix idempotency not found error during agent startup
- Fix bug that could cause unexpected behavior during parameter replacement in document
- Gather metrics during agent version validation in Windows agent update
- Make long sleep for onprem same as long sleep for EC2, and cap sleep time at 30 minutes for OnPrem instances
- Migrated snap package builder from core18 to core22
- Parse version from OS release file correctly when contains special chars
- Suppress logs from the go-routine that checks the session manager's orchestration directory
- Update go git dependency to v5.12.0
- Update seelog config to have default time format with Milliseconds
- Update TMP/TEMP env variable during windows installer launch in Updater
- Upgrade GoLang to version 1.21.12

3.3.551.0
===============
- Agent updater attempts yum install/uninstall before falling back to attempt with rpm
- Updated golang.org/x/net from v0.19.0 to v0.26.0
- Upgrade GoLang to version 1.21.11
- Add IPv6 addresses for NTP and EC2Config to default DenyList
- Update Distributor to only use Systems Manager APIs to fetch package contents

3.3.484.0
===============
- Update SSM-Setup-CLI logs related to checksum validation of latest version

3.3.418.0
===============
- Upgrade go-github version from v8 to v61
- Increase timeouts in SSM-Setup-CLI
- Fix darwin build issue in SSM-Setup-CLI
- Fix the command builder bug to handle space char in input value
- Fix an inaccurate log when validating allowDowngrade parameter during Agent update
- Signing SSM Agent vended Windows executables

3.3.380.0
===============
- Update AWS GO SDK to v1.51.20

3.3.337.0
===============
- Remove yum as package manager in linux install/uninstall script
- Verify TrustedInstaller status before posting WindowsUpdate information in aws:softwareInventory plugin

3.3.217.0
===============
- Add alternative outputs for agent package generation scripts
- Add support for Oracle 8.8 & 8.9, Rocky 8.8 & 8.9, AlmaLinux 8.8 & 8.9, and RHEL 8.9 & 9.3
- Fix flaky integration test
- Fix setup-cli Darwin build issue
- Fix setup-cli error code for non English systems
- Set IPR creds expiry to 30 mins for ssm agent worker
- Switch installer package manager from rpm to yum on OSes that support yum
- Upgrade GoLang to version 1.21.8

3.3.131.0
===============
- Add integration tests for control channel and data channel module
- Remove data channel and control channel acknowledgement functionality in MGS Interactor

3.3.40.0
===============
- Fix issue to execute aws:updateSSMAgent plugin through aws:rundocument plugin
- Update Messaging module to switch off ec2messages when ssmmessages connected successfully
- Update SSM Agent Minor version from 3.2 to 3.3

3.2.2222.0
===============
- Upgrade minimum go version in go.mod file to go 1.19
- Upgrade go-git package to v5.11.0
- Fix for bad default manifest url when updating EC2Config

3.2.2143.0
===============
- Fixed plugin path traversal logic
- Updated aws:application plugin default param
- Fixed default param in psmodule
- Upgraded GoLang to version 1.21.5

3.2.2086.0
===============
- Added Agent config to configure session logs destination
- Added data channel acknowledgement functionalities
- Added redirect handler and timeout for HTTP client
- Added steps to verify aws-cli installation for domainJoin plugin
- Added support for Ubuntu 23.04, Debian 11.7 & 12, and SUSE 15.5
- Adjusted random number generator logic used to get filename in downloadContent plugin
- Fixed Agent to gather application inventory from both rpm and dpkg package managers if present in Unix instances
- Bump golang.org/x/crypto/ssh from 0.14.0 to 0.17.0
- Added EXCLUDE\_INTERFACES environment variable support to exclude certain interfaces

3.2.2016.0
===============
- Added telemetry for agent core in-proc executor usage
- Added retries for Agent installation with snap on Greengrass 
- Added code to update Agent config to use only Onprem Identity in Greengrass
- Added support for macOS 14 (Sonoma)
- Added Onprem registration support using ssm-setup-cli
- Fixed docker installation issues in aws:configureDocker plugin
- Fix for document worker and session worker not logging when custom seelog configuration missing parameters
- Updated allowed regex pattern in S3 URI
- Update Agent IoT Greengrass component minor version
- Updated SUSE version in Seamless Domain Join script
- Updated Greengrass component workflow to get installed Agent version and update Agent only when the installed Agent version doesn't match with Greengrass component Agent version
- Upgraded GoLang version that builds agent binaries with to 1.20.11

3.2.1798.0
===============
- Bump golang.org/x/net from 0.15.0 to 0.17.0
- Upgraded GoLang to version 1.20.10
- Fixing race condition in session datachannel unit test

3.2.1705.0
===============
- Updated MGS Interactor to send 'Failed' status on agentJob parsing error
- Added error handling for Linux DomainJoin when service account credentials empty
- Fix for panic scenario in when running aws:configureDocker plugin
- Upgraded GoLang to version 1.20.8
- Upgraded golang.org/x/net to v0.15.0
- Added support for macOS 13 (Ventura)

3.2.1630.0
===============
- Fix credential retrieval retry logic in credential refresher
- Reducing retrieval log level to debug in the credential refresher after more than 3 retrieval retries
- Fix for EC2 credential retrieval errors not being propagated to the credential refresher
- Fixing agent version input format validation
- Fix downloadPlatformOverride for AlmaLinux
- Fixed issue where removing seelog.xml file doesn't revert minimum log level back to INFO
- Ignore non-audit files in audit folder

3.2.1542.0
===============
- Add aws:updateSSMAgent plugin support for Flatcar Linux
- Add fix to resolve manifest url during agent update when using stable keyword
- Fix multiple issues causing tight loops during IPC connection scenarios
- Sign deb and rpm installer packages for Linux instances using new key
- Use file based IPC by default for amazon-ssm-agent and ssm-agent-worker communication in Darwin

3.2.1478.0
===============
- Added fix to propagate exit code properly when command fails to start
- Added control channel acknowledgement functionalities
- Added flag to specify go version used for gosec and govulncheck in static analysis script
- Added support for RHEL 8.7, 8.8, 9.1, 9.2
- Added support for Rocky Linux 8.7, 9.0, 9.1, 9.2
- Added support for Oracle Linux 8.7, 9.1, 9.2
- Update go version to 1.20.7

3.2.1377.0
===============
- Stopped saving instance profile credentials to disk
- Added static agent security scans to makefile
- Updated Greengrass component minor version

3.2.1297.0
===============
- Added retries to snap uninstall call in setupcli
- Fix for windows shutdown executable not found when compiled with golang1.19+
- Fix to return correct Agent Job ID for ack after AgentJobParseError
- Pass golang contexts for network calls in agent core to terminate cleanly
- Remove credential file dependency in agent workers implemented in 3.2.x.x versions
- Report MGS Connection Channel status to Health table
- Update Dockerfile to use Golang image from ECR repository

3.2.1241.0
===============
- Get bucket region using signed HeadBucket request
- Updated golang.org/x/net version to 0.10.0 and golang.org/x/crypto version to 0.9.0

3.2.1041.0
===============
- Add retry to handle stream data acknowledge messages
- Support latest as a version in configurePackage plugin
- Updated AWS GO SDK to v1.44.261 and disabled IMDSv1 fallback logic
- Use IP address to connect to destination server in port session

3.2.985.0
===============
- Add Domain Join support for RHEL 8.7 and AL2022
- Add Support to send aws:updateSSMAgent replies through MGS
- Retrieve and set interface name dynamically in aws:domainJoin plugin for Ubuntu

3.2.923.0
===============
- Update Dockerfile Go version to 1.19
- Add reporting of MGS connection status
- Add support for updating to agent version marked stable
- Add status code to MGS ack and send on message process failure
- Update golangci-lint configuration
- Add e2e tag to session shell tests

3.2.815.0
===============
- Add EC2 credential fallback for AssumeRoleUnauthorizedAccess error
- Add CloudWatch log upload support for document and session worker
- Add set-hostname support in domainjoin plugin for windows 
- Add wait time in Agent updater to avoid installation issues caused during reboots initiated by domainjoin plugin
- Add support for AlmaLinux
- Fix KeepHostName parameter without DNS IP address parameter in domainJoin plugin
- Fix issue where carriage returns cause json conversion to fail in aws:softwareInventory plugin
- Remove IMDS calls in Onprem during health check
- Remove S3 global endpoint fallback logic
- Update cli descriptions for registration parameters
- Update go version to 1.19.6

3.2.582.0
===============
- Modified EC2 credential fallback logic

3.2.574.0
===============
- Fixed go-vet issues by passing mocks by value
- Updated domainjoin and cloudwatch executables for windows

3.2.532.0
===============
- Removed explicit setting of EC2 aws credential profile
- Added public key to registration info
- Sends non-interactive command errors that occur before command execution to data channel
- Added instance id verification to registration process

3.2.419.0
===============
- Added minimum retry sleep for Registrar RegisterManagedInstance calls
- Explicitly skip AZ info check for on-prem and ECS targets
- Fix for SSM-Agent that is unable to start on Apple Mac M1's (mac2.metal instances)
- Ensuring powershell path is set to system directory on Windows
- Load DLLs with using system/absolute paths on Windows
- Added workaround for Samba limit when loading Active Directory ids
- Dynamically get network interface name for SeamlessDomainJoin
- Added install-yum-rpm to makefile to install agent on host from source code
- Added logging for specifying credential source
- Refactored tests to remove mocks from production binaries
- Updated Windows DomainJoin plugin SharpZipLib and Newtonsoft.json dependencies

3.2.345.0
===============
- Updated yaml.v3 dependency

3.2.286.0
===============
- Separated EC2 identity vault manifest from OnPrem identity vault manifest
- Fix for credential retrieval blocking os termination signals
- Fix for agent updater using shared credentials on EC2
- Added guards against panic for agent identity health checks
- Added logging around agent module start/stop

3.2.183.0
===============
- Added logging when assuming identity
- Increased retries to ECS metadata endpoint
- Added linux debug build to makefile
- Implemented aws sdk logging interface
- Updated agent minor version to 3.2
- Added functionality to retrieve agent credentials from Systems Manager on EC2

3.1.1927.0
===============
- Update shell for Session Manager on MacOS

3.1.1856.0
===============
- Lower message length threshold for cloudwatch log streaming
- Ran gofmt and goimports with golang version 1.19
- Report AvailabilityZone and AvailabilityZoneId in health pings
- Update AWS Go SDK to v1.44.78

3.1.1767.0
===============
- Fix samba configuration for sub-domains

3.1.1732.0
===============
- Add code in document/session worker to fallback to default identity selector when runtime config not present
- Fix to handle command-line-arguments in document/session worker when launched by old agent workers

3.1.1634.0
===============
- Fallback to file based IPC if named pipe creation times out
- Increase tls handshake timeout in http download client
- Log mds client timeout errors as WARN

3.1.1575.0
===============
- Added separate metric for snapd running apps failure during update
- Fixed idle session timeout with smux keep alive configuration based on CLI version
- Updated AgentTaskComplete message retry
- Updated go version to 1.18.3

3.1.1511.0
===============
- Collect kernel version in InstanceDetailedInformation
- Support separate output stream for non-interactive session
- Cleanup default log group name for runcommands
- Updated rpm spec file to include build id

3.1.1476.0
===============
- Fix port session premature close when local server is not connected before timeout

3.1.1446.0
===============
- Add created date to AgentJobAck message
- Disable smux keep alive to use idle session timeout feature
- Fix unit-tests running on windows

3.1.1374.0
===============
- Added timeout for s3 HEAD requests
- Added vpc address deny to port forwarding
- Fixed for reboot scenario in configure package plugin
- Fixed goroutine leak in seelog library
- Fixed nullpointer segmentation fault in configure package plugin
- Improved error handling in manifest download in updater
- Improved worker initialization to improve startup failure logging

3.1.1260.0
===============
- Added missing check for invalid S3 path parameter
- Added support for domain join using a non-local username
- Fixed broken links in README.md
- Fixed ECS Exec issue where agent was using environment variables for credentials
- Updated Ec2Detector test to query smbios directly for system information

3.1.1208.0
===============
- Updated ec2detector module to use Get-CmiInstance instead of wmic.exe
- Fixed file creation mode of ssm-agent-users sudoer file

3.1.1188.0
===============
- Added new ec2detector module to determine if agent is on EC2
- Added support for port forwarding to remote host
- Added quotes around inventory parameter ValueName on Windows
- Fix for domain join DNS IP assignments in shared directories
- Replaced namedpipe updater test with ec2detector test

3.1.1141.0
===============
- Add application inventory by file for Bottlerocket
- Fix infinite retry logic to send failed replies in MGSInteractor
- Remove usage of io/fs package

3.1.1080.0
===============
- (windows only) Remove symlink scan during update

3.1.1045.0
===============
- Fixed sourceHash validation for aws:application document plugin
- Added document parameter validation for values passed to target document of aws:runDocument plugin
- (windows only) Fix process leak when legacy cloudwatch plugin is enabled
- (windows only) Fail installation if C:\ProgramData\Amazon\SSM\ has symlinks

3.1.1004.0
===============
- Added platform detection for Bottlerocket OS
- Consolidated regional endpoint generation to common endpoint module

3.1.941.0
===============
- Added support for Rocky linux
- Fixed sharefile/shareprofile not being propagated to updateutil
- Fixed incorrect darwin platform detection post BigSur
- Fixed log flush issue in updater
- Updated .NET dependencies for domainjoin and cloudwatch (windows only)
- Updated go version to 1.17.6

3.1.821.0
===============
- Implement new core module named MessageService to start processing commands from both MGS and MDS
  - Merge functionalities from RunCommandService core module and Session core module.
  - Receive run command documents through MGS if connected and fallback to MDS otherwise. This functionality requires appropriate permissions for both endpoints and will be rolled out gradually to end users.
  - Provide filesystem based idempotency check to avoid duplicate run command document execution.
  - Increase default run command pool buffer size from 1 to 5 to load additional documents before-hand for processing.
- Fix nil pointer deference panic produced in named pipe test case during agent update
- Remove StopType concept in ssm-agent-worker and add different waits for reboot and shutdown stop

3.1.804.0
===============
- Add support for upstart when running get-diagnostic command using ssm-cli
- Fix systemctl service name to support older versions of systemctl
- Include changes to facilitate testing
- Update DNS server selection logic for seamless domain join on linux and darwin
- Update go version to go1.17.5
- Update golang sys package dependency

3.1.715.0
===============
- Derive default directories from appconfig on Darwin
- Set x-bit on newly-created directories

3.1.634.0
===============
- Fix for ssm-setup-cli to be able to select service manager without the agent being installed

3.1.630.0
===============
- Added greengrass component recipe for the new SystemsManagerAgent component
- Added support for registering agent on a greengrass device
- Added support for downloading more than 1000 objects in downloadContent
- Fixed retry logic for onprem and s3 upload
- Fixed unit tests when running on Mac
- Update AWS SDK to v1.41.4
- Update logic to retrieve platform details for Rocky Linux

3.1.501.0
===============
- Add diagnostics command to ssm-cli
- Fix caching for onprem credentials
- Additional configuration options for Seamless Domain Join
- Gracefully exit session if group of runas user is modified
- Skip retries for cert validation errors in S3 HEAD requests
- Fix DNS failures on CentOS 8.2
- Update several dependencies

3.1.459.0
===============
- Fixed a bug with powershell command for Inventory

3.1.426.0
===============
- Fixed cpu spike issue manifesting on snap
- Fixed issue with version comparison in EC2Config update plugin
- Fixed panic when command output was being truncated
- Updated build to use go1.16.8
- Removed Profile from inventory powershell commands on Windows

3.1.338.0
===============
- Fix to eliminate WaitGroup reuse panic triggered during agent reboot
- Fix to include applications without UninstallString in Inventory for Windows
- Fixed a bug where multi-plugin documents with large outputs would timeout RunCommand
- Fixed a bug where RunCommand could delay executions for up to 15 minutes

3.1.282.0
===============
- Add serial port logging of AwsNitroEnclaves package version on windows during startup
- Allow usage of existing loggroup/logstream when the user does not have create permission
- Change service interrogate request log to debug
- Cleanup old surveyor channel files on startup
- Fix filehandle leak in windows leading to agent going offline
- Fix to schedule correct next run time during orchestration directories cleanup
- Fix to sequentially update correct runcount value in the document bookkeeping file
- Fix a bug with version parsing EC2Config updater
- Updated rpm packaging for fips compliance

3.1.192.0
===============
- Added darwin arm64 to makefile
- Added logic to limit orchestration directory cleanup
- Added packaging for public SSM Agent container image
- Fixed cloudwatch endpoint for telemetry metrics requests
- Fixed handling of Windows filepaths and mutex locks
- Fixed agent worker handling of OS signals and termination channel requests
- Updated datachannel retry strategy to not retry for a specific error scenario
- Updated default gomaxproc value for Windows
- Update build to use go1.16.6

3.1.127.0
===============
- Added a workaround for windows random halts
- Fixed race condition during reboot document execution

3.1.90.0
===============
- Updated to version 3.1
- Updated build to build statically linked binaries for linux 64bit
  - Minimum supported linux kernel version for linux 64bit is 3.2+
- Fixed permissions for docker config file
- Fixed issue with ubuntu prerm and postinst scripts
- Fixed issue where processor stop was being called twice

3.0.1390.0
===============
- Added config option to delete orchestration folder
- Added snapcraft packaging config
- Added workaround for aws:runDocument status bug
- Added improved handling of file closure
- Added support for go mod and updated build to use go 1.16.4
- Fixed bug parsing vpce s3 urls
- Refactored use of agent identity in agent cli
- Updated check if agent is running as windows service
- Updated handling of session cancellation to still send output to client side
- Updated interactive session exit code logic to match non-interactive mode
- Updated vendor dependencies

3.0.1295.0
===============
- Added configurable custom identity and identity consumption order
- Added cross-account domain join
- Added cleanup for older versions of updater artifacts
- Added a workaround for MacOS kernel bug that sometimes kept RunCommand from launching
- Added a workaround for log file contention on Windows
- Added synchronization to RunCommand service stop
- Changed hibernation log level  
- MacOS executables are now signed
- Removed delay in non-interactive session type

3.0.1209.0
===============
- Fixed issue where registration file is not removed when registration is cleared
- Removed unnecessary CloudWatch Log api calls
- Added support for IMDSv2 in Windows AD domain join plugin

3.0.1181.0
===============
- Added support for digest authorization in downloadContent plugin
- Added missing defer close for windows service in updater
- Added support to disable onprem hardware similarity check
- Fixed windows random halts issue
- Refactored windows startup
- Refactored task pool to dynamically dispatch goroutines

3.0.1124.0
===============
- Added a check for broken symlink after update
- Added support for NonInteractiveCommands session type on Linux and Windows platforms
- Added lint-all flag to makefile
- Changed Inventory plugin billinginfo to use IMDSv2
- Fixed indefinite retries for ResourceError during CWLogging
- Fixed go vet call in checkstyle.sh
- Fixed inter process communication log line
- Fixed a bug where CloudWatch logs were not being uploaded
- Fixed timer and goroutine leaks
- Fixed an issue where document workers on Windows were not exiting

3.0.1031.0
===============
- Added test-all flag to the makefile
- Added support for onprem private key auto rotation
- Added config to remove plugin output files after upload to s3
- Added update precondition for upcoming 3.1 release
- Fixed cloudwatch windows where TLS 1.0 is disabled
- Fixed document cloudwatch upload when CreateLogStream permissions were missing left instances stuck in terminating
- Fixed domain join windows EC2 instances where TLS 1.0 is disabled
- Fixed domain join script for .local domain names
- Fixed domain join script to exit when domain is already joined
- Fixed panic issue in windows startup script when executing powershell command
- Fixed session manager issue on MacOS for root and home path
- Removed IMDS call in domain join script
- Refactored update plugin and updater interaction

3.0.882.0
===============
- Added jitter to first control channel call
- Added dedicated folder for plugins
- Added option to overwrite corrupt shared credentials

3.0.854.0
===============
- Added $HOME env variable for root user when runAsElevated is true in session
- Added CREAD flag in serial port control flags on linux
- Added PlatformName and PlatformVersion as env variables for aws:runShellScript
- Added support for macOS updater
- Added v2.2 document support in updater
- Added defer recover statements
- Fixed inventory error log when dpkg is not available
- Fixed ssm-cli logging to stdout
- Removed consideration of unimportant error codes in service side
- Updated ec2 credential caching time to ~1 hour
- Updated service query logic for Windows
- Updated golang sys package dependency

3.0.755.0
===============
- Fix fallback logic for MGS endpoint generation
- Fix regional endpoint generation

3.0.732.0
===============
- Fix bug in document parameter expansion
- Fix datachannel to wait for empty message buffer before closing
- Fix for hung Session Manager sessions
- Fix for folder permission issue in domain join
- Refactor identity handling
- Update session plugin to pause reading when datachannel not actively sending data
- Update ssm-user creation details in README.md

3.0.655.0
===============
- Add feature to retain hostname during domain join
- Add delay to pty start failure for session-worker
- Add nil pointer check on shell command for session-worker
- Add shlex to vendor which is used to parse session interactive command input for session-worker
- Change log level for IPC not readable message
- Change v2 agent to use v3 agent executor
- Fix network connectivity issues on RHEL8
- Fix race condition where first message is dropped when session plugin's message handler is not ready
- Fix file channel protocol test cases
- Fix blocking http call when certificates are not available
- Move aws cli installation out of /tmp for domain join plugin
- Update boolean attributes in Session Document to accept both string and bool values
- Upgrade vendor dependencies and build to use go1.15.7

3.0.603.0
===============
- Added instruction to README.md for getting the latest version of SSM Agent in a specific region
- Fix for PowerShell stream data being executed in reverse order
- Fix to create update lock folder before creating update locks
- Fix to reset ipcTempFile properties at the end of session

3.0.529.0
===============
- Fix for encrypted s3 bucket upload

3.0.502.0
===============
- Add agent version flag to retrieve agent version
- Add onFailure/onSuccess/finallyStep support for plugins
- Add SSE header for S3 Upload
- Add SSM Agent support in MacOS
- Extend use of default http transport
- Fix for Agent not aquiring new instance role credentials after EC2 hibernation
- Fix for shell profile powershell commands not being executed in the expected order
- Fix to delete undeleted channel while using reboot document
- Fix to consider status of all plugin steps in document after system restart
- Fix bug capturing rpm install exit code
- Handle sourceInfo json sent from CLI in downloadContent plugin
- Optimize agent startup time by removing additional wait times 
- Refactor makefile
- Replace master branch with mainline branch
- Upgrade aws-sdk-go to latest version(v1.35.23)

3.0.431.0
===============
- Use DefaultTransport as underlying RoundTripper for S3 access

3.0.413.0
===============
- Add additional checks and logs to install scripts
- Add retry logic to handle ssm document during reboot
- Add dockerfile to build agent
- Add script to package binaries to tar
- Change default download directory on Linux to /var/lib/amazon/ssm
- Extend SSM Agent ability to execute from relative path and use custom certificates
- Fix IP address parsing in domain join plugin
- Fix self update logging
- Log fingerprint similarity check failures as ERROR and each changed machine property as WARN
- Prefix ecs target id with 'ecs:'
- Prefer non-link-local addresses to show in Console
- Use IMDSv1 after IMDSv2

3.0.356.0
===============
- Fail update document if updater fails to execute
- Fallback to file-based IPC if named pipes are not available
- Add support for streaming of logs to CloudWatch for Session Manager
- Add support for following cross-region redirects from S3
- Refactor .deb and .rpm packaging scripts
- Fix intermittent test failures
- Search full path for valid sc.exe
- Log PV driver version on Windows instances
- Add -trimpath to build flags

3.0.284.0
===============
- Added steps to the updater to validate IPC functionality
- Added SSM_COMMAND_ID environment variable to runShellScript plugin
- Improved retry for S3 and http(s) downloads
- Fix to clean up terminated worker processes
- Fix for DomainJoin when using domain-ou

3.0.222.0
===============
- Added agent and worker version logging
- Added new config parameters to README.md
- Added support for TCP multiplexing in port plugin
- Fix for s3Upload to retry with an exponential backoff when uploading logs
- Fix for startup modules to handle panic
- Fix for systemd configuration to always restart the agent when it exits for any reason 

3.0.196.0
===============
- Add support for document parameters in document plugin’s preconditions
- Add support for new source types in aws:downloadContent plugin: HTTP(S) endpoints and private Git repositories
- Add support for Session Manager configurable shell profile
- Fix parsing of irregular inventory version strings
- Fix error handling for windows wmi service
- Fix to stop BillingInfo call for OnPremise systems
- Fix to correct OS parsing map for openSUSE leap platform in configurePackage plugin
- Fix to treat timed out docs in SuccessAndReboot state as failed

3.0.161.0
===============
- Fix install scripts to report errors from package manager and enable retries

3.0.151.0
===============
- First release of SSM Agent v3
- Moved v2 amazon-ssm-agent to new ssm-agent-worker binary
- New amazon-ssm-agent binary:
  - Opt-in self update feature to upgrade if agent is running a deprecated version
  - Telemetry feature to send important audit events to AWS. Opt-in send to customer CloudWatch
  - Monitor and keep the ssm-agent-worker process running
- Upgrade vendor dependencies and build to use go1.13

2.3.1644.0
===============
- Enable aws:domainJoin SSM API for Linux
- Sanitize platform name

2.3.1613.0
===============
- Adjust retry settings for update operations
- Fix Session manager initialization issue
- Fix deserialization issue in configurePackage plugin

2.3.1569.0
===============
- Add code to cleanup interim documents if unable to parse
- Bug fix for executing StartProcess module in a goroutine to avoid blocking the main thread

2.3.1550.0
===============
- Add retry to install/uninstall during update
- Bug fix in updater logging
- Bug fix for object downloads from s3
- Support passing additional arguments to Distributor script execution

2.3.1509.0
===============
- Add retry to s3 downloads during update
- Add support for large inventory items
- Add update lock so only one update can execute at a time
- Bug fix for cross region s3 upload
- Bug fix for github build 
- Bug fixes for CloudWatch logs
- Updated packaging dependencies

2.3.1319.0
===============
- Updated README.md to include amazon-ssm-agent.json config definitions
- Bug fix for reporting ConfigurePackage metrics for document archive
- Added backoff for CloudWatch retries
- Cleaned up error codes and handling dependency errors
- Bug fix for logging http response status when dialing of websocket connection fails

2.3.1205.0
===============
- Updated the SSM Agent Snap to core18
- Bug fix for expired in-progress documents being resumed
- Bug fix for update specific files not being deleted after agent update is finished
- Bug fix for cached manifest files not being deleted in the configurepackage plugin

2.3.978.0
===============
- Stop pty on receiving TerminateSession request
- Add support for Debian arm64 architecture
- Refactoring session log generation logic

2.3.930.0
===============
- Bug fix for CloudWatch agent version showing twice in Inventory console
- Bug fix for retrieving minor version for CentOS7
- Add snap appData collection for inventory in ubuntu 18
- Add validation for contents of os release files
- Add retry for fingerprint generation

2.3.871.0
===============
- Various bug fix for SSM Agent

2.3.842.0
===============
- Bug fix for updating document state file prior agent reboot
- Add support to restart agent after SIGPIPE exit status

2.3.814.0
===============
- Bug fix for metadata service V2
- Update Golang version 1.12 for travis
- Optimize session manager retry logic 

2.3.786.0
===============
- Add support for Oracle Linux v7.5 and v7.7
- Bug fix for Inventory data provider to support special characters
- Bug fix for SSM MDS service name

2.3.772.0
===============
- Upgrade AWS SDK
- Add logging for fingerprint generation 
 
2.3.760.0
===============
- Session manager supports handling of Task metadata

2.3.758.0
===============
- Add support to update SSM Distributor packages in place

2.3.756.0
===============
- Terminate port forwarding session on receiving TerminateSession flag
- Bug fix to reload SSM client if region has not been initialize correctly
- Bug fix for retrieval of user groups on Linux 

2.3.722.0
===============
- Bug fix for the delay when registering non-EC2 on-prem instances
- Bug fix for missing ACL when uploading logs to S3 buckets
- Upgrade GoLang version from 1.9 to 1.12

2.3.714.0
===============
- For port forwarding session, close server connection when client drops it's connection
- Bug fix for missing condition of rules from inventory registry
- Update service domain information fetch logic from EC2 Metadata

2.3.707.0
===============
- Bug fix for characters dropping from session manager shell output 
- Bug fix for session manager freezing caused by non utf8 character
- Switch the request protocol order for getting S3 Header
- Keep port forwarding session open until session is terminated

2.3.701.0
===============
- Send platform type information in controlChannel input 

2.3.687.0
===============
- Bug fix for runPowershellScript plugin on linux platform
- Add support for document 2.x version to ssm-cli 

2.3.680.0
===============
- Added a new Inventory gatherer AWS:BillingInfo which will gather the billing product ids for LicenseIncluded and Marketplace instance

2.3.672.0
===============
- Add Port plugin for SSH/SCP
- Add support for Session Manager RunAs functionality on Linux platform

2.3.668.0
===============
- Add Session Manager InteractiveCommands plugin	
- Bug fix for log formatting issue for session manager

2.3.662.0
===============
- Bug fix for Session Manager when handling line endings on Windows platform
- Bug fix for token validation for aws:downloadContent plugin
- Check if log group exists before uploading Session Manager logs to CloudWatch
- Bug fix for broken S3 urls when using custom documents

2.3.634.0
===============
- Disable appconfig to load credential from specific profile path, add EC2 credentials as the default fallback
- Remove sudoers file creation logic if ssm-user already exists
- Enable supplementary groups for ssm-user on Linux

2.3.612.0
===============
- Bug fix for UTF-8 encoded issue caused by locale activation on Ubuntu 16.04 instance
- Refactor ssm-user creation logic
- Bug fix for reporting IP address with wrong network interface
- Update configure package document arn pattern

2.3.542.0
===============
- Bug fix for on-premises instance registration in CN region

2.3.539.0
===============
- Add support for further encryption of session data using AWS KMS
- Bug fix for excessive instance-id fetching by document workers

2.3.479.0
===============
- Bug fix for downloading content failure caused by wrong S3 endpoint
- Bug fix for reboot failure caused by session manager panic
- Bug fix for session manager shell output dropping character
- Bug fix for mgs endpoint configuration consistency

2.3.444.0
===============
- Updates to UpdateInstanceInformation call, Windows initialization

2.3.415.0
===============
- Bug fix addressing issues in Distributor package upgrade

2.3.372.0
===============
- Bug fix to allow installation of Distributor packages that do not have a version name.
- Bug fix for agent crash with message "WaitGroup is reused before previous Wait has returned".

2.3.344.0
===============
- Add frequent collector to detect changed inventory types and upload it to SSM service between two scheduled collections.
- Change AWS Systems Manager Distributor to reduce calls to GetDocument by calling DescribeDocument.
- Add exit code when ssm-cli execution fails.
- Create ssm-user only  after the control channel has been successfully created.

2.3.274.0
===============
- Enabled AWS Systems Manager Distributor that lets you securely distribute and install software packages.
- Add support for the arm64 architecture on Amazon Linux 2, Ubuntu 16.04/18.04, and RHEL 7.6 to support EC2 A1 instances.

2.3.235.0
===============
- Bug fix for session manager logging on Windows
- Bug fix for ConfigureCloudWatch plugin
- Bug fix for update SSM agent occasionally failing due to SSM agent service stuck in starting state

2.3.193.0
===============
- Bug fix for past sessions occasionally stuck in terminating state
- Darwin masquerades as Linux to bypass OS validation on the backend until official support can be added

2.3.169.0
===============
- Update managed instance role token more frequently

2.3.136.0
===============
- Bug fix for issue that GatherInventory throw out error when there is no Windows Update in instance
- Add more filters when getting the Windows event logs at startup to improve performance
- Add random jitter before call PutInventory in inventory datauploader

2.3.117.0
===============
- Bug fix for issues during process termination on instances where IAM policy does not grant ssmmessages permissions.

2.3.101.0
===============
- Bug fix to prevent defunct processes when creating the local user ssm-user.
- Bug fix for sudoersFile permission to avoid "sudo" command warnings in Session Manager.
- Disable hibernation on Windows platform if Cloudwatch configuration is present.

2.3.68.0
===============
- Enables the Session Manager capability that lets you manage your Amazon EC2 instance through an interactive one-click browser-based shell or through the AWS CLI.
- Beginning this agent version, SSM Agent will create a local user "ssm-user" and either add it to /etc/sudoers (Linux) or to the Administrators group (Windows) every time the agent starts. The ssm-user is the default OS user when a Session Manager session is started, and the password for this user is reset on every session. You can change the permissions by moving the ssm-user to a less-privileged group or by changing the sudoers file. The ssm-user is not removed from the system when SSM Agent is uninstalled.

2.3.13.0
===============
- Bug fix for the SSM Agent service remaining in "Starting" state on Windows when unable to authenticate to the Systems Manager service.

2.2.916.0
===============
NOTE: This build should not be installed for Windows since the SSM Agent service may remain in starting status if unable to authenticate to the Systems Manager service, which is fixed in the latest release.
- Bug fix for missing cloudwatch.exe seen in SSM Agent version 2.2.902.0

2.2.902.0
===============
NOTE: This build should not be installed for Windows since you might see the error - "Encountered error while starting the plugin: Unable to locate cloudwatch.exe" for Cloudwatch plugin. This bug has been fixed in SSM Agent version 2.2.916.0. Also SSM Agent service may remain in starting status if unable to authenticate to the Systems Manager service, which is fixed in the latest release.
- Initial support for developer builds on macOS
- Retry sending Run Command execution results for up to 2 hours
- More detailed error messages are returned for inventory plugin failures during State Manager association executions

2.2.800.0
===============
- Bug fix to clean the orchestration directory
- Streaming AWS Systems Manager Run Command output to CloudWatch Logs
- Reducing number of retries for serial port opening 
- Add retry logic to installation verification

2.2.619.0
===============
- Various bug fixes

2.2.607.0
===============
- Various bug fixes

2.2.546.0
===============
- Bug fix to retry sending document results if they couldn't reach the service

2.2.493.0
===============
NOTE: Downgrade to this version using AWS-UpdateSSMAgent is not permitted for agent installed using snap
- Added support for Ubuntu Snap packaging
- Bug fix so that aws:downloadContent does not change permissions of directories
- Bug fix to Cloudwatch plugin where StartType has duplicated Enabled value

2.2.392.0
===============
- Added support for agent hibernation so that Agent backs off or enters hibernation mode if it does not have access to the service
- Various bug fixes

2.2.355.0
===============
- Fix S3Download to download from cross regions.
- Various bug fixes

2.2.325.0
===============
- Bug fix to change sourceHashType to be default sha256 on psmodule.
- Various bug fixes

2.2.257.0
===============
- Bug fix to address an issue that can prevent the agent from processing associations after a restart

2.2.191.0
===============
- Various bug fixes.

2.2.160.0
===============
- Fix bug on windows agent (v2.2.103, v2.2.120.0) running into hung state with high volume of associations/runcommands
- Execute "pwsh" on linux when using runPowershellScript plugin

2.2.136.0
===============
NOTE: There is a known issue in this build that can cause Windows instances to stop processing commands and associations. The issue is resolved in build versions>2.2.136.0
- Various bug fixes.

2.2.120.0
===============
NOTE: There is a known issue in this build that can cause Windows instances to stop processing commands and associations. The issue is resolved in build versions>2.2.136.0   
- Various bug fixes.

2.2.103.0
===============
NOTE: There is a known issue in this build that can cause Windows instances to stop processing commands and associations. The issue is resolved in build versions>2.2.136.0
- Various bug fixes.

2.2.93.0
===============
- Update to latest AWS SDK.
- Various bug fixes.

2.2.82.0
===============
- Bug fix for proxy environment variables in Windows.

2.2.64.0
===============
- Various bug fixes.

2.2.58.0
===============
- Switching to use Birdwatcher distribution service for AWS packages.
- Various bug fixes.

2.2.45.0
===============
- Adding versioning support for Parameter Store.
- Added additional gatherers for inventory, including windows service gatherer, windows registry gatherer, file metadata gatherer, windows role gatherer.
- Added support for aws:downloadContent plugin to download content from GitHub, S3 and documents from SSM documents.
- Added support for aws:runDocument plugin to execute SSM documents.

2.2.30.0
===============
- Improved speed of initial association application on boot
- Various aws:configurePackage service integration changes
- Improved home directory detection in non-x64 linux platforms to address cases where shared AWS SDK credentials were not available in on-prem instances

2.2.24.0
===============
- Added exponential backoff in bucket region check for s3 upload
- Fixed an issue with orchestration directory cleanup for RunCommand
- Important Reminder: In an upcoming release, the RPM installer won't start the service by default after initial RPM
  installation. All customers should update any automation for RPM-based installs to start the agent after install if desired.

2.2.16.0
===============
- Increment major/minor version to 2.2
- Bug fix on update to v2.1.10.0
- Advance Notice: In an upcoming release, to align with the guidelines for RPM-based distros, the RPM installer won't
  start the service by default after initial RPM installation. It will start automatically after RPM update only if the
  service was running previously. The behavior of the Debian and MSI installer is unchanged.

2.1.10.0
===============
- Including SSM-CLI in Debian 386 packages
- Bug fix for multi-step document output
- Various bug fixes

2.1.4.0
===============
- Support for command execution out-of-process

2.0.952.0
===============
- Various bug fixes

2.0.922.0
===============
- Added Raspbian support for armv6 to support Raspberry Pi
- Various bug fixes

2.0.913.0
===============
- Updated golang/sys dependency to the latest
- Increased run command document maximum execution timeout to 48 hours
- Various bug fixes

2.0.902.0
===============
- Added support for uploading agent logs to CloudWatch for SSM Agent diagnostics
- Added additional gatherers for inventory
- Added configuration compliance support for association
- Various bug fixes

2.0.879.0
===============
- Add capability to configure custom s3 endpoint for the agent
- Various bug fixes

2.0.847.0
===============
- Various bug fixes

2.0.842.0
===============
- Added rollback support in aws:configurePackage
- Various bug fixes

2.0.834.0
===============
- Various bug fixes

2.0.822.0
===============
- [Bug] This version is not a valid update target from version 2.0.761.0.
- Added support for using the OS proxy settings by default in Windows
- Fixed issue preventing CloudWatch proxy settings from being retained on update
- Various bug fixes

2.0.805.0
===============
- Added support for SLES (SuSE) (64-bit, v12 and above)
- Various bug fixes

2.0.796.0
===============
- Linux platform version now based on os-release when available
- Various bug fixes

2.0.790.0
===============
- Added support for step-level preconditions
- Added support for rate/interval based schedule expressions for associations
- Added Summary and PackageID fields to inventory's aws:application gatherer
- Changed inventory's aws:application gatherer to use win32_processor: addressWidth to detect OS architecture
  to avoid localization based errors
- Fixed CloudWatch issue with large configuration
- Fixed S3 upload when instance and bucket are not in the same region
- Fixed bug that prevented native language AMIs (Japanese AMI) from launching Cloudwatch
- Various bug fixes

2.0.767.0
===============
- Returning longer StandardOutput and StandardError from RunShellScript and RunPowerShellScript
  which show up in the results of GetCommandInvocation and the detailed output of ListCommandInvocation
- Added Document v2.0 support for Run Command, which includes support for multiple actions of same plugin type
- Various bug fixes

2.0.761.0
===============
- Amazon-ssm-agent service automatically started after reboot on systemd platforms
- Added Release notes to be available on linux packages
- Various bug fixes

2.0.755.0
===============
- Fixed bugs that prevented CloudWatch from launching and allowed multiple instances of CloudWatch to launch on Windows
- Various bug fixes

2.0.730.0
===============
- Fixed issues with agent starting before network is ready on systemd systems.

2.0.716.0
================
- Pass proxy settings to domain join and CloudWatch
- Various bug fixes

2.0.706.0
================
- Various bug fixes

2.0.682.0
================
- Added support for installing Docker on Linux
- Removed the upper limit for the maximum number of parallel executing documents on the agent (previously the max was 10)
You can configure this number by setting the “CommandWorkerLimit” attribute in amazon-ssm-agent.json file

2.0.672
================
- Added bucket-owner-full-control ACL to S3 outputs to support cross-account upload
- Various bug fixes

2.0.660
================
- Various bug fixes
- Standardized S3 result paths across plugins; commands append command-id/instance-id/plugin-name/step-id
  associations append instance-id/association-id/execution-date/plugin-name/step-id
  * step-id is the id field in plugin input if present and supported, otherwise the step name (in 2.0 schema documents), otherwise the plugin-name again
  * plugin-name and step-id have : characters removed
- FreeBSD patches from external contributor

2.0.633
================
- Added support for aws:softwareInventory plugin to upload inventory related log messages to S3
- Fixed CloudWatch crash issue
- Various bug fixes

2.0.617
================
- Fixed Domain Join to support customized OU

2.0.599
================
- Added support for running Powershell on Linux
- Fixed CloudWatch doesn't work with creating association from Console
- Various bug fixes

2.0.571
================
- Various bug fixes

2.0.562
================
- Fixed SSM Agent not able to start on Windows Server 2003
- Various bug fixes

2.0.558
================
- Various bug fixes

2.0.533
================
- Added support for State Manager that automates the process of keeping your Amazon EC2 and hybrid infrastructure in a state that you define
You can use State Manager to ensure that your instances are bootstrapped with specific software at startup, configured according to your security policy, joined to a Windows domain, or patched with specific software updates throughout their lifecycle
- Added support for Systems Manager Inventory that allows you to specify the type of metadata to collect, the instances from where the metadata should be collected, and a schedule for metadata collection
- Added support for installing, uninstalling, and updating AWS packages published by AWS
- Added support for installing Docker on Windows and running Docker actions

1.2.371
================
- Added support for Amazon EC2 Simple Systems Manager (SSM) Config feature to manage the configuration of your instances while they are running.
You create an SSM document, which describes configuration tasks (for example, installing software), and then associate the SSM document with one or more running instances
- Added support for Windows Server 2016
- Added support for Windows Server Nano

1.2.298
================
- Various bug fixes

1.2.290
================
- Added support for Ubuntu Xenial (16.04 LTS)
- Added support for region cn-north-1

1.2.252
================
- Added support for allowing Amazon EC2 Run Command to work on any instance or virtual machine outside of AWS, including your own data centers or other clouds
You now have a consistent experience to extend your scripts across locations and automate administrative tasks across instances, irrespective of location

1.1.0
================
- Added addition platform (CentOS, Ubuntu) support
- Added 32bits support

1.0.178
================
- Initial SSM Agent release
