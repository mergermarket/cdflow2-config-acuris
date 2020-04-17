package handler

import (
	"io"
	"os"

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
	ErrorStream      io.Writer
}

// AWSClientFactory takes AWS env vars to create its client.
type AWSClientFactory func(env map[string]string) AWSClient

// New returns a new handler.
func New(awsClientFactory AWSClientFactory, errorStream io.Writer) *Handler {
	return &Handler{
		AWSClientFactory: awsClientFactory,
		ErrorStream:      errorStream,
	}
}

// NewWithDefaults returns a handler with default values.
func NewWithDefaults() *Handler {
	return &Handler{
		AWSClientFactory: getAWSClientSDKFactory(),
		ErrorStream:      os.Stderr,
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
