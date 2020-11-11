#!/usr/bin/env bash
echo "****************************************"
echo "Copying DomainJoin and CloudWatch binaries"
echo "****************************************"

BIN_FOLDER="$GO_SPACE/bin/"

CLOUDWATCH_FOLDER="${BIN_FOLDER}awsCloudwatch/"
SESSION_MANAGER_SHELL_FOLDER="${BIN_FOLDER}SessionManagerShell/"

PLUGIN_DEPENDENCIES="$GO_SPACE/packaging/dependencies"

# Set DomainJoin path
DJ_DEPENDENCIES=$PLUGIN_DEPENDENCIES

# Copy DomainJoin dependencies
cp $DJ_DEPENDENCIES/AWS.DomainJoin.exe $BIN_FOLDER
cp $DJ_DEPENDENCIES/AWS.DomainJoin.exe.config $BIN_FOLDER
cp $DJ_DEPENDENCIES/log4net.config $BIN_FOLDER

# Set CloudWatch path
CW_DEPENDENCIES="$PLUGIN_DEPENDENCIES/awsCloudwatch"

# Prepare CloudWatch folder
if [[ -d "$CLOUDWATCH_FOLDER" ]]; then
   rmdir "$CLOUDWATCH_FOLDER"
fi
mkdir "$CLOUDWATCH_FOLDER"

# Copy CloudWatch dependencies
cp $CW_DEPENDENCIES/AWS.CloudWatch.exe $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWS.CloudWatch.exe.config $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWS.CloudWatch.log4net.config $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.CloudWatch.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.CloudWatchLogs.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.Core.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.EC2Messaging.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWS.EC2.Windows.CloudWatch.Configuration.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWS.EC2.Windows.CloudWatch.DataFlowParser.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWS.EC2.Windows.CloudWatch.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.S3.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/AWSSDK.SimpleSystemsManagement.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Common.Logging.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Ec2Config.Common.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Ec2Config.Ec2ConsoleLogger.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Ec2Config.Plugin.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Ec2Config.Plugin.Internal.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Ec2Config.Plugin.Tools.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/ICSharpCode.SharpZipLib.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Jetbrains.Annotations.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/log4net.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.EnterpriseLibrary.Common.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.EnterpriseLibrary.Validation.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.ServiceLocation.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.Unity.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.Unity.Configuration.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Microsoft.Practices.Unity.Interception.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Newtonsoft.Json.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/Quartz.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/SmartThreadPool.dll $CLOUDWATCH_FOLDER
cp $CW_DEPENDENCIES/System.Threading.dll $CLOUDWATCH_FOLDER

# Set Session path
SESSION_DEPENDENCIES="$PLUGIN_DEPENDENCIES"

# Prepare Session folder
if [[ -d "$SESSION_MANAGER_SHELL_FOLDER" ]]; then
   rmdir "$SESSION_MANAGER_SHELL_FOLDER"
fi
mkdir "$SESSION_MANAGER_SHELL_FOLDER"

# Copy Session dependencies
if [ -f "$SESSION_DEPENDENCIES/winpty/winpty.dll" ]; then
  cp $SESSION_DEPENDENCIES/winpty/winpty.dll $SESSION_MANAGER_SHELL_FOLDER
  cp $SESSION_DEPENDENCIES/winpty/winpty-agent.exe $SESSION_MANAGER_SHELL_FOLDER
fi