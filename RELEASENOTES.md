Latest
===============
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
