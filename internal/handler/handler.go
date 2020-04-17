package handler

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	common "github.com/mergermarket/cdflow2-config-common"
)

const AccountID = "724178030834"
const Region = "eu-west-1"
const DefaultLambdaBucket = "acuris-lambdas"

// Handler handles config requests.
type Handler struct {
	STSClientFactory STSClientFactory
	OutputStream     io.Writer
	ErrorStream      io.Writer
}

// Opts are the options for creating a new handler.
type Opts struct {
	STSClientFactory STSClientFactory
	OutputStream     io.Writer
	ErrorStream      io.Writer
}

// STSClientFactory is a factory method for creating an STS client.
type STSClientFactory func(env map[string]string) (stsiface.STSAPI, error)

// New returns a new handler.
func New(opts *Opts) *Handler {
	factory := opts.STSClientFactory
	if factory == nil {
		factory = getSTSClientFactory()
	}
	return &Handler{
		STSClientFactory: factory,
		OutputStream:     opts.OutputStream,
		ErrorStream:      opts.ErrorStream,
	}
}

func getSTSClientFactory() STSClientFactory {
	return func(env map[string]string) (stsiface.STSAPI, error) {
		id := env["AWS_ACCESS_KEY_ID"]
		secret := env["AWS_SECRET_ACCESS_KEY"]
		if id == "" || secret == "" {
			return nil, fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not found in env")
		}
		creds := credentials.NewStaticCredentials(id, secret, env["AWS_SESSION_TOKEN"])
		session, err := session.NewSession(aws.NewConfig().WithCredentials(creds).WithRegion("eu-west-1"))
		if err != nil {
			return nil, fmt.Errorf("unable to create a new AWS session: %v", err)
		}
		return sts.New(session), nil
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
