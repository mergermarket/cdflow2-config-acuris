package handler

import (
	"io"

	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

const AccountID = "724178030834"
const Region = "eu-west-1"
const DefaultLambdaBucket = "acuris-lambdas"

type AWSClient interface {
	STSAssumeRole(teamName string) (*sts.Credentials, error)
	GetECRRepoURI(componentName string) (string, error)
}

// Handler handles config requests.
type Handler struct {
	AWSClientFactory AWSClientFactory
	OutputStream     io.Writer
	ErrorStream      io.Writer
}

// Opts are the options for creating a new handler.
type Opts struct {
	AWSClientFactory AWSClientFactory
	OutputStream     io.Writer
	ErrorStream      io.Writer
}

// AWSClientFactory takes AWS env vars to create its client.
type AWSClientFactory func(env map[string]string) AWSClient

// New returns a new handler.
func New(opts *Opts) *Handler {
	awsClientFactory := opts.AWSClientFactory
	if awsClientFactory == nil {
		awsClientFactory = getAWSClientSDKFactory()
	}
	return &Handler{
		AWSClientFactory: awsClientFactory,
		OutputStream:     opts.OutputStream,
		ErrorStream:      opts.ErrorStream,
	}
}

// Setup sets up the project.
func (handler *Handler) Setup(request *common.SetupRequest, response *common.SetupResponse) error {
	return nil
}

// UploadRelease runs after release to upload the release.
func (handler *Handler) UploadRelease(request *common.UploadReleaseRequest, response *common.UploadReleaseResponse, version string, config map[string]interface{}) error {
	return nil
}

// PrepareTerraform runs before terraform to configure.
func (handler *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse) error {
	return nil
}
