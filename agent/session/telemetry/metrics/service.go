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
	cont "context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	stopPolicyErrorThreshold = 10
	stopPolicyName           = "CloudWatchService"
	maxRetries               = 3
)

// ICloudWatchService is the interface to create and push cloud watch metrics
type ICloudWatchService interface {
	GenerateUpdateMetrics(metricName string, value float64, sourceVersion string, targetVersion string) *cwtypes.MetricDatum
	GenerateBasicTelemetryMetrics(metricName string, value float64, version string) *cwtypes.MetricDatum
	PutMetrics(metricData []*cwtypes.MetricDatum) error
	IsCloudWatchEnabled() bool
}

// CloudWatchService encapsulates the client and stop policy as a wrapper to call the CloudWatch API
type CloudWatchService struct {
	context           context.T
	service           *cloudwatch.Client
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
func (c *CloudWatchService) GenerateUpdateMetrics(metricName string, value float64, sourceVersion string, targetVersion string) *cwtypes.MetricDatum {
	return &cwtypes.MetricDatum{
		MetricName: aws.String(metricName),
		Unit:       cwtypes.StandardUnitCount,
		Value:      aws.Float64(value),
		Dimensions: []cwtypes.Dimension{
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
func (c *CloudWatchService) GenerateBasicTelemetryMetrics(metricName string, value float64, version string) *cwtypes.MetricDatum {
	return &cwtypes.MetricDatum{
		MetricName: aws.String(metricName),
		Unit:       cwtypes.StandardUnitCount,
		Value:      aws.Float64(value),
		Dimensions: []cwtypes.Dimension{
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
func (c *CloudWatchService) PutMetrics(metricData []*cwtypes.MetricDatum) error {
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

	metricDataItems := make([]cwtypes.MetricDatum, len(metricData))
	for i, item := range metricData {
		if item != nil {
			metricDataItems[i] = *item
		}
	}

	output, err := c.service.PutMetricData(cont.TODO(), &cloudwatch.PutMetricDataInput{
		MetricData: metricDataItems,
		Namespace:  &c.namespace,
	})

	if err != nil {
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
func (c *CloudWatchService) createCloudWatchClient() *cloudwatch.Client {
	config := sdkutil.AwsConfig(c.context, "monitoring")

	config.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = maxRetries + 1
		})
	}

	appConfig := c.context.AppConfig()
	config.APIOptions = append(config.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("AddUserAgent", func(ctx cont.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req := in.Request.(*smithyhttp.Request)
				userAgent := fmt.Sprintf("%s/%s", appConfig.Agent.Name, appConfig.Agent.Version)
				req.Header.Add("User-Agent", userAgent)
				return next.HandleBuild(ctx, in)
			}),
			middleware.After,
		)
	})

	return cloudwatch.NewFromConfig(config)
}
