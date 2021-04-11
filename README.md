[![ReportCard][ReportCard-Image]][ReportCard-URL]
[![Build Status](https://travis-ci.org/aws/amazon-ssm-agent.svg?branch=mainline)](https://travis-ci.org/aws/amazon-ssm-agent)

# Amazon SSM Agent

The Amazon EC2 Simple Systems Manager (SSM) Agent is software developed for the [Simple Systems Manager Service](http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html). The SSM Agent is the primary component of a feature called Run Command.

## Overview

The SSM Agent runs on EC2 instances and enables you to quickly and easily execute remote commands or scripts against one or more instances. The agent uses SSM [documents](http://docs.aws.amazon.com/ssm/latest/APIReference/aws-ssm-document.html). When you execute a command, the agent on the instance processes the document and configures the instance as specified.
Currently, the agent and Run Command enable you to quickly run Shell scripts on an instance using the AWS-RunShellScript SSM document. 
SSM Agent also enables the Session Manager capability that lets you manage your Amazon EC2 instance through an interactive one-click browser-based shell or through the AWS CLI. The first time a Session Manager session is started on an instance, the agent will create a user called "ssm-user" with sudo or administrator privilege. Session Manager sessions will be launched in context of this user.

### Verify Requirements

* [SSM Run Command Prerequisites](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/remote-commands-prereq.html)
* [SSM Session Manager Prerequisites and supported Operating Systems](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-prerequisites.html)

### Setup

* [Configuring IAM Roles and Users for SSM Run Command](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ssm-iam.html)
* [Configuring the SSM Agent](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/install-ssm-agent.html)
* [Configuring IAM Roles for Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-instance-profile.html)
* [Configuring Users for Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-restrict-access.html)

### Executing Commands

[SSM Run Command Walkthrough Using the AWS CLI](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/walkthrough-cli.html)

### Starting Sessions

[Session Manager Walkthrough Using the AWS Console and CLI](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-sessions-start.html)

### Troubleshooting

[Troubleshooting SSM Run Command](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/troubleshooting-remote-commands.html)
[Troubleshooting SSM Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-troubleshooting.html)

## Feedback

Thank you for helping us to improve Systems Manager, Run Command and Session Manager. Please send your questions or comments to [Systems Manager Forums](https://forums.aws.amazon.com/forum.jspa?forumID=185&start=0)

### Building inside docker container (Recommended)
* Install docker: [Install CentOS](https://docs.docker.com/engine/install/centos/)

* Build image
```
docker build -t ssm-agent-build-image .
```
* Build the agent
```
docker run -it --rm --name ssm-agent-build-container -v `pwd`:/amazon-ssm-agent ssm-agent-build-image make build-release
```

### Building on Linux

* Install go [Getting started](https://golang.org/doc/install)

* Install rpm-build and rpmdevtools

* [Cross Compile SSM Agent](https://www.ardanlabs.com/blog/2013/10/cross-compile-your-go-programs.html)

* Run `make build` to build the SSM Agent for Linux, Debian, Windows environment.

* Run `make build-release` to build the agent and also packages it into a RPM, DEB and ZIP package.

The following folders are generated when the build completes:
```
bin/debian_386
bin/debian_amd64
bin/linux_386
bin/linux_amd64
bin/linux_arm
bin/linux_arm64
bin/windows_386
bin/windows_amd64
```
* To enable the Agent for Session Manager scenario on Windows instances
    * Clone the repo from https://github.com/masatma/winpty.git
    * Follow instructions on https://github.com/rprichard/winpty to build winpty 64-bit binaries
    * Copy the winpty.dll and winpty-agent.exe to the bin/SessionManagerShell folder
For the Windows Operating System, Session Manager is only supported on Windows Server 2008 R2 through Windows Server 2019 64-bit versions.

Please follow the user guide to [copy and install the SSM Agent](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/install-ssm-agent.html)

### Code Layout

* Source code
    * Core functionality such as worker management is under core/
    * Agent worker code is under agent/
    * Other functionality such as IPC is under common/
* Vendor package source code is under vendor/src
* rpm and dpkg artifacts are under packaging
* build scripts are under Tools/src

### Linting

To lint the entire module call the `lint-all` target. This executes golangci-lint on all packages in the module.
You can configure golangci-lint with different linters using the `.golangci.yml` file.

For golangci-lint installation instructions see https://golangci-lint.run/usage/install/
For more information on the golangci-lint configuration file see https://golangci-lint.run/usage/configuration/
For more information on the linters used see https://golangci-lint.run/usage/linters/

### GOPATH

To use vendor dependencies, the suggested GOPATH format is `:<packagesource>/vendor:<packagesource>`

### Make Targets

The following targets are available. Each may be run with `make <target>`.

| Make Target              | Description |
|:-------------------------|:------------|
| `build`                  | *(Default)* `build` builds the agent for Linux, Debian, Darwin and Windows amd64 and 386 environment |
| `build-release`          | `build-release` checks code style and coverage, builds the agent and also packages it into a RPM, DEB and ZIP package |
| `release`                | `release` checks code style and coverage, runs tests, packages all dependencies to the bin folder. |
| `package`                | `package` packages build result into a RPM, DEB and ZIP package |
| `pre-build`              | `pre-build` goes through Tools/src folder to make sure all the script files are executable |
| `checkstyle`             | `checkstyle` runs the checkstyle script |
| `quick-integtest`        | `quick-integtest` runs all tests tagged with integration using `go test` |
| `quick-test`             | `quick-test` runs all the tests including integration and unit tests using `go test` |
| `coverage`               | `coverage` runs all tests and calculate code coverage |
| `build-linux`            | `build-linux` builds the agent for execution in the Linux amd64 environment |
| `build-windows`          | `build-windows` builds the agent for execution in the Windows amd64 environment |
| `build-darwin`           | `build-darwin` builds the agent for execution in the Darwin amd64 environment |
| `build-linux-386`        | `build-linux-386` builds the agent for execution in the Linux 386 environment |
| `build-windows-386`      | `build-windows-386` builds the agent for execution in the Windows 386 environment |
| `build-darwin-386`       | `build-darwin-386` builds the agent for execution in the Darwin 386 environment |
| `build-arm`              | `build-arm` builds the agent for execution in the arm environment |
| `build-arm64`            | `build-arm64` builds the agent for execution in the arm64 environment |
| `lint-all`               | `lint-all` runs golangci-lint on all packages. golangci-lint is configured by .golangci.yml |
| `package-rpm`            | `package-rpm` builds the agent and packages it into a RPM package for Linux amd64 based distributions |
| `package-deb`            | `package-deb` builds the agent and packages it into a DEB package Debian amd64 based distributions |
| `package-win`            | `package-win` builds the agent and packages it into a ZIP package Windows amd64 based distributions |
| `package-rpm-386`        | `package-rpm-386` builds the agent and packages it into a RPM package for Linux 386 based distributions |
| `package-deb-386`        | `package-deb-386` builds the agent and packages it into a DEB package Debian 386 based distributions |
| `package-win-386`        | `package-win-386` builds the agent and packages it into a ZIP package Windows 386 based distributions |
| `package-rpm-arm64`      | `package-rpm-arm64` builds the agent and packages it into a RPM package Linux arm64 based distributions |
| `package-deb-arm`        | `package-deb-arm` builds the agent and packages it into a DEB package Debian arm based distributions |
| `package-deb-arm64`      | `package-deb-arm64` builds the agent and packages it into a DEB package Debian arm64 based distributions |
| `package-linux`          | `package-linux` create update packages for Linux and Debian based distributions |
| `package-windows`        | `package-windows` create update packages for Windows based distributions |
| `package-darwin`         | `package-darwin` create update packages for Darwin based distributions |
| `get-tools`              | `get-tools` gets gocode and oracle using `go get` |
| `clean`                  | `clean` removes build artifacts |

### Contributing

Contributions and feedback are welcome! Proposals and Pull Requests will be considered and responded to. Please see the [CONTRIBUTING.md](https://github.com/aws/amazon-ssm-agent/blob/mainline/CONTRIBUTING.md) file for more information.

Amazon Web Services does not currently provide support for modified copies of this software.

## Runtime Configuration

To set up your own custom configuration for the agent:
* Navigate to /etc/amazon/ssm/ (or C:\Program Files\Amazon\SSM for windows)
* Copy the contents of amazon-ssm-agent.json.template to a new file amazon-ssm-agent.json
* Restart agent

### Config Property Definitions:
* Profile - represents configurations for aws credential profile used to get managed instance role and credentials
    * ShareCreds (boolean)
        * Default: true
    * ShareProfile (string)
    * ForceUpdateCreds (boolean) - overwrite shared credentials file if existing one cannot be parsed
        * Default: false
    * KeyAutoRotateDays (int) - defines the maximum age in days for on-prem private key, default value might change to 30 in the close future
        * Default: 0 (never rotate)
* Mds - represents configuration for Message delivery service (MDS) where agent listens for incoming messages
    * CommandWorkersLimit (int)
        * Default: 5
    * StopTimeoutMillis (int64)
        * Default: 20000
    * Endpoint (string)
    * CommandRetryLimit (int)
        * Default: 15
* Ssm - represents configuration for Simple Systems Manager (SSM)
    * Endpoint (string)
    * HealthFrequencyMinutes (int)
        * Default: 5
    * CustomInventoryDefaultLocation (string)
    * AssociationLogsRetentionDurationHours (int)
        * Default: 24
    * RunCommandLogsRetentionDurationHours (int)
        * Default: 336
    * SessionLogsRetentionDurationHours (int)
        * Default: 336
    * PluginLocalOutputCleanup (string) - Configure when after execution it is safe to delete local plugin output logs in orchestration folder
        * Default: "" - Don't delete logs immediately after execution. Fall back to AssociationLogsRetentionDurationHours, RunCommandLogsRetentionDurationHours, and SessionLogsRetentionDurationHours 
        * OptionalValue: "after-execution" - Delete plugin output file locally after plugin execution
        * OptionalValue: "after-upload" - Delete plugin output locally after successful s3 or cloudWatch upload
* Mgs - represents configuration for Message Gateway service
    * Region (string)
    * Endpoint (string)
    * StopTimeoutMillis (int64)
        * Default: 20000
    * SessionWorkersLimit (int)
        * Default: 1000
* Agent - represents metadata for amazon-ssm-agent
    * Region (string)
    * OrchestrationRootDir (string)
        * Default: "orchestration"
    * SelfUpdate (boolean)
        * Default: false
    * TelemetryMetricsToCloudWatch (boolean)
        * Default: false
    * TelemetryMetricsToSSM (boolean)
        * Default: true
    * AuditExpirationDay (int)
        * Default: 7
    * LongRunningWorkerMonitorIntervalSeconds (int)
        * Default: 60
* Os - represents os related information, will be logged in reply messages
    * Lang (string)
        * Default: "en-US"
    * Name (string)
    * Version (string)
        * Default: 1
* S3 - represents configurations related to S3 bucket and key for SSM. Endpoint and region are typically determined automatically, and should only be set if a custom endpoint is required.  LogBucket and LogKey are currently unused.
    * Endpoint (string)
        * Default: ""
    * Region (string) - Ignored
    * LogBucket (string) - Ignored
    * LogKey (string) - Ignored
* Kms - represents configuration for Key Management Service if encryption is enabled for this session (i.e. kmsKeyId is set or using "Port" plugin) 
    * Endpoint (string)

## Release

After the SSM Agent source code has been released to github, it can take up to 2 weeks for the install packages to propagate to all AWS regions.

The following commands can be used to pull the `VERSION` file and check the latest agent available in a region.
* Regional Bucket *(Non-CN*) - `curl https://s3.{region}.amazonaws.com/amazon-ssm-{region}/latest/VERSION`
  * Replace `{region}` with region code like `us-east-1`.
* Regional Bucket *(CN)* - `curl https://s3.{region}.amazonaws.com.cn/amazon-ssm-{region}/latest/VERSION`
  * Replace `{region}` with region code `cn-north-1`, `cn-northwest-1`.
* Global Bucket - `curl https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/VERSION`

## License

The Amazon SSM Agent is licensed under the Apache 2.0 License.

[ReportCard-URL]: http://goreportcard.com/report/aws/amazon-ssm-agent
[ReportCard-Image]: http://goreportcard.com/badge/aws/amazon-ssm-agent
