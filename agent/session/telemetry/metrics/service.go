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

// metrics is responsible for pulling logs from the log queue and publishing them to cloudwatch

package metrics

import (
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	stopPolicyErrorThreshold = 10
	stopPolicyName           = "CloudWatchService"
	maxRetries               = 3
)

// ICloudWatchService is the interface to create and push cloud watch metrics
type ICloudWatchService interface {
	GenerateUpdateMetrics(metricName string, value float64, sourceVersion string, targetVersion string) *cloudwatch.MetricDatum
	GenerateBasicTelemetryMetrics(metricName string, value float64, version string) *cloudwatch.MetricDatum
	PutMetrics(metricData []*cloudwatch.MetricDatum) error
	IsCloudWatchEnabled() bool
}

// CloudWatchService encapsulates the client and stop policy as a wrapper to call the CloudWatch API
type CloudWatchService struct {
	context           context.T
	service           *cloudwatch.CloudWatch
	stopPolicy        *sdkutil.StopPolicy
	namespace         string
	instanceId        string
	cloudWatchEnabled bool
}

// NewCloudWatchService Creates a new instance of the CloudWatchService
func NewCloudWatchService(context context.T) *CloudWatchService {
	instance, err := context.Identity().InstanceID()
	if err != nil {
		context.Log().Error("failed to get the instance id, %v", err)
	}

	cloudWatchService := CloudWatchService{
		context:           context,
		stopPolicy:        createCloudWatchStopPolicy(),
		namespace:         context.AppConfig().Agent.TelemetryMetricsNamespace,
		instanceId:        instance,
		cloudWatchEnabled: context.AppConfig().Agent.TelemetryMetricsToCloudWatch,
	}
	cloudWatchService.service = cloudWatchService.createCloudWatchClient()

	if !cloudWatchService.cloudWatchEnabled {
		context.Log().Info("agent telemetry cloudwatch metrics disabled")
	}
	return &cloudWatchService
}

// IsCloudWatchEnabled returns whether the agent telemetry to cloud watch is enabled or not
func (c *CloudWatchService) IsCloudWatchEnabled() bool {
	return c.cloudWatchEnabled
}

// GenerateUpdateMetrics generate metrics with instance id, TargetVersion and SourceVersion as the dimension
func (c *CloudWatchService) GenerateUpdateMetrics(metricName string, value float64, sourceVersion string, targetVersion string) *cloudwatch.MetricDatum {
	return &cloudwatch.MetricDatum{
		MetricName: aws.String(metricName),
		Unit:       aws.String("Count"),
		Value:      aws.Float64(value),
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: aws.String(c.instanceId),
			},
			{
				Name:  aws.String("TargetVersion"),
				Value: aws.String(targetVersion),
			},
			{
				Name:  aws.String("SourceVersion"),
				Value: aws.String(sourceVersion),
			},
		},
	}
}

// GenerateBasicTelemetryMetrics generate metrics with instance id and AgentVersion as the dimension
func (c *CloudWatchService) GenerateBasicTelemetryMetrics(metricName string, value float64, version string) *cloudwatch.MetricDatum {
	return &cloudwatch.MetricDatum{
		MetricName: aws.String(metricName),
		Unit:       aws.String("Count"),
		Value:      aws.Float64(value),
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: aws.String(c.instanceId),
			},
			{
				Name:  aws.String("AgentVersion"),
				Value: aws.String(version),
			},
		},
	}
}

// PutMetrics publishes the metrics to CloudWatch
func (c *CloudWatchService) PutMetrics(metricData []*cloudwatch.MetricDatum) error {
	log := c.context.Log()
	if !c.cloudWatchEnabled {
		return errors.New("agent telemetry cloudwatch metrics disabled")
	}
	log.Infof("Reporting agent telemetry metrics")
	log.Debugf("metric data, %v", metricData)
	if !c.stopPolicy.IsHealthy() {
		c.service = c.createCloudWatchClient()
		c.stopPolicy.ResetErrorCount()
	}

	putRequest, output := c.service.PutMetricDataRequest(&cloudwatch.PutMetricDataInput{
		MetricData: metricData,
		Namespace:  &c.namespace,
	})

	if err := putRequest.Send(); err != nil {
		sdkutil.HandleAwsError(log, err, c.stopPolicy)

		return err
	}

	log.Debugf("PutMetricDataRequest Response, %v", output)
	return nil
}

// createCloudWatchStopPolicy creates a new policy for CloudWatch
func createCloudWatchStopPolicy() *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(stopPolicyName, stopPolicyErrorThreshold)
}

// createCloudWatchClient creates a client to call CloudWatchLogs APIs
func (c *CloudWatchService) createCloudWatchClient() *cloudwatch.CloudWatch {
	config := sdkutil.AwsConfig(c.context, "monitoring")

	config = request.WithRetryer(config, client.DefaultRetryer{
		NumMaxRetries: maxRetries,
	})

	appConfig := c.context.AppConfig()
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	return cloudwatch.New(sess)
}
