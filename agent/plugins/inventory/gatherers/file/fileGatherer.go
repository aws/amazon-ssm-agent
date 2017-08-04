package file

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName captures name of file gatherer
	GathererName = "AWS:File"
	// SchemaVersionOfFileGatherer represents schema version of file gatherer
	SchemaVersionOfFileGatherer = "1.0"
)

type T struct{}

// Gatherer returns new file gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

var collectData = collectFileData

// Name returns name of file gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes file gatherer and returns list of inventory.Item comprising of file data
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {

	var result model.Item

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z to comply with regex at SSM.
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)
	var data []model.FileData
	data, err = collectData(context, configuration)

	result = model.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfFileGatherer,
		Content:       data,
		CaptureTime:   captureTime,
	}

	items = append(items, result)
	return
}

// RequestStop stops the execution of application gatherer.
func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}
