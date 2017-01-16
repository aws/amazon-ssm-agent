Latest
================
- Added support for aws:softwareInventory plugin to upload inventory related log messages to S3
- Fixed CloudWatch crash issue
- Various bug fixes

2.0.671
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