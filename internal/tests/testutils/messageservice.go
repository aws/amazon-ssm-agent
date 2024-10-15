package testutils

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mdsinteractor"
	mds "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
)

// NewMessageService creates message service module that can be injected into core module
func NewMessageService(context context.T, mdsService mds.Service) contracts.ICoreModule {
	interactors := make([]interactor.IInteractor, 0)

	// create new message service module
	messageServiceCoreModule := messageservice.NewService(context)
	messageServiceRef := messageServiceCoreModule.(*messageservice.MessageService)

	// create new mds interactor
	interactorRef, _ := mdsinteractor.New(context, messageServiceRef.GetMessageHandler(), mdsService)
	interactors = append(interactors, interactorRef)
	// add mds interactor with mock mds service to the message service
	messageServiceRef.SetInteractor(interactors)

	return messageServiceCoreModule
}
