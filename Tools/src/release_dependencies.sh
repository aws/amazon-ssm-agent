#!/usr/bin/env bash
echo "********************************************"
echo "Release  Binary Dependencies for Open Source"
echo "********************************************"

PACKAGING_FOLDER="$BGO_SPACE/packaging/dependencies/"
PLUGIN_BINARIES="`brazil-path run.configfarm.artifacts`/artifacts"
CLOUDWATCH_FOLDER="$BIN_FOLDER/awsCloudwatch/"

cp $PLUGIN_BINARIES/AWS.DomainJoin.exe $PACKAGING_FOLDER
cp $PLUGIN_BINARIES/AWS.DomainJoin.exe.config $PACKAGING_FOLDER
cp $PLUGIN_BINARIES/log4net.config $PACKAGING_FOLDER

cp $PLUGIN_BINARIES/AWS.CloudWatch.exe $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWS.CloudWatch.exe.config $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWS.CloudWatch.log4net.config $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.CloudWatch.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.CloudWatchLogs.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.Core.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.EC2Messaging.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWS.EC2.Windows.CloudWatch.Configuration.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWS.EC2.Windows.CloudWatch.DataFlowParser.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWS.EC2.Windows.CloudWatch.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.S3.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/AWSSDK.SimpleSystemsManagement.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Common.Logging.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Ec2Config.Common.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Ec2Config.Ec2ConsoleLogger.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Ec2Config.Plugin.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Ec2Config.Plugin.Internal.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Ec2Config.Plugin.Tools.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/ICSharpCode.SharpZipLib.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Jetbrains.Annotations.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/log4net.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.EnterpriseLibrary.Common.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.EnterpriseLibrary.Validation.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.ServiceLocation.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.Unity.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.Unity.Configuration.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Microsoft.Practices.Unity.Interception.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Newtonsoft.Json.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/Quartz.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/SmartThreadPool.dll $CLOUDWATCH_FOLDER
cp $PLUGIN_BINARIES/System.Threading.dll $CLOUDWATCH_FOLDER