[![ReportCard][ReportCard-Image]][ReportCard-URL]
[![Build Status](https://travis-ci.org/aws/amazon-ssm-agent.svg?branch=master)](https://travis-ci.org/aws/amazon-ssm-agent)

# Amazon SSM Agent

The Amazon EC2 Simple Systems Manager (SSM) Agent is software developed for the [Simple Systems Manager Service](http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html). The SSM Agent is the primary component of a feature called Run Command.

## Overview

The SSM Agent runs on EC2 instances and enables you to quickly and easily execute remote commands or scripts against one or more instances. The agent uses SSM [documents](http://docs.aws.amazon.com/ssm/latest/APIReference/aws-ssm-document.html). When you execute a command, the agent on the instance processes the document and configures the instance as specified.
Currently, the agent and Run Command enable you to quickly run Shell scripts on an instance using the AWS-RunShellScript SSM document. 
SSM Agent also enables the Session Manager capability that lets you manage your Amazon EC2 instance through an interactive one-click browser-based shell or through the AWS CLI. When the agent starts, it will create a user called "ssm-user" with sudo or administrator privilege. Session Manager sessions will be launched in context of this user.

### Verify Requirements

[SSM Run Command Prerequisites](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/remote-commands-prereq.html)
[SSM Session Manager Prerequisites and supported Operating Systems](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-prerequisites.html)

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
  
## Building and Running from source

* Install go [Getting started](https://golang.org/doc/install)

* Install rpm-build
```
sudo yum install -y rpmdevtools rpm-build
```

* [Cross Compile SSM Agent](http://www.goinggo.net/2013/10/cross-compile-your-go-programs.html)

* Run `make build` to build the SSM Agent for Linux, Debian, Windows environment.

* Run `make release` to build the agent and also packages it into a RPM, DEB and ZIP package.

The following folders are generated when the build completes:
```
bin/debian_386
bin/debian_amd64
bin/linux_386
bin/linux_amd64
bin/windows_386
bin/windows_amd64
```
* To enable the Agent for Session Manager scenario on Windows instances
    * Clone the repo from https://github.com/masatma/winpty.git
    * Follow instructions on https://github.com/rprichard/winpty to build winpty 64-bit binaries
    * Copy the winpty.dll and winpty-agent.exe to the bin/SessionManagerShell folder
For the Windows Operating System, Session Manager is only supported on Windows Server 2008 R2 through Windows Server 2016 64-bit versions.

Please follow the user guide to [copy and install the SSM Agent](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/install-ssm-agent.html)

### Code Layout

* Source code is under agent/
* Vendor package source code is under vendor/src
* rpm and dpkg artifacts are under packaging
* build scripts are under Tools/src

### GOPATH

To use vendor dependencies, the suggested GOPATH format is `:<packagesource>/vendor:<packagesource>`

### Make Targets

The following targets are available. Each may be run with `make <target>`.

| Make Target              | Description |
|:-------------------------|:------------|
| `build`                  | *(Default)* `build` builds the agent for Linux, Debian and Windows amd64 and 386 environment |
| `release`                | `release` checks code style and coverage, builds the agent and also packages it into a RPM, DEB and ZIP package |
| `package`                | `package` packages build result into a RPM, DEB and ZIP package |
| `pre-build`              | `pre-build` goes through Tools/src folder to make sure all the script files are executable |
| `checkstyle`             | `checkstyle` runs the checkstyle script |
| `quick-integtest`        | `quick-integtest` runs all tests tagged with integration using `go test` |
| `quick-test`             | `quick-test runs all the tests including integration and unit tests using `go test` |
| `coverage`               | `coverage` runs all tests and calculate code coverage |
| `build-linux`            | `build-linux` builds the agent for execution in the Linux amd64 environment |
| `build-windows`          | `build-windows` builds the agent for execution in the Windows amd64 environment |
| `build-darwin`           | `build-darwin` builds the agent for execution in the Darwin amd64 environment |
| `build-linux-386`        | `build-linux-386` builds the agent for execution in the Linux 386 environment |
| `build-windows-386`      | `build-windows-386` builds the agent for execution in the Windows 386 environment |
| `build-darwin-386`       | `build-darwin-386` builds the agent for execution in the Darwin 386 environment |
| `create-rpm`             | `create-rpm` builds the agent and packages it into a RPM package for Linux amd64 based distributions|
| `create-deb`             | `create-deb` builds the agent and packages it into a DEB package Debian amd64 based distributions|
| `create-win`             | `create-win` builds the agent and packages it into a ZIP package Windows amd64 based distributions|
| `create-rpm-386`         | `create-rpm-386` builds the agent and packages it into a RPM package for Linux 386 based distributions|
| `create-deb-386`         | `create-deb-386` builds the agent and packages it into a DEB package Debian 386 based distributions|
| `create-win-386`         | `create-win-386` builds the agent and packages it into a ZIP package Windows 386 based distributions|
| `create-linux-package`   | `create-linux-package` create update packages for Linux and Debian based distributions|
| `create-windows-package` | `create-windows-package` create update packages for Windows based distributions|
| `get-tools`              | `get-tools` gets gocode and oracle using `go get` |
| `clean`                  | `clean` removes build artifacts.|

### Contributing

Contributions and feedback are welcome! Proposals and Pull Requests will be considered and responded to. Please see the [CONTRIBUTING.md](https://github.com/aws/amazon-ssm-agent/blob/master/CONTRIBUTING.md) file for more information.

Amazon Web Services does not currently provide support for modified copies of this software.

## License

The Amazon SSM Agent is licensed under the Apache 2.0 License.

[ReportCard-URL]: http://goreportcard.com/report/aws/amazon-ssm-agent
[ReportCard-Image]: http://goreportcard.com/badge/aws/amazon-ssm-agent
