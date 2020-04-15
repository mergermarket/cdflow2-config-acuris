package handler

import (
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	common "github.com/mergermarket/cdflow2-config-common"
)

// Handler handles config requests.
type Handler struct {
	STSClient stsiface.STSAPI
}

// Opts are the options for creating a new handler.
type Opts struct {
	STSClient stsiface.STSAPI
}

// New returns a new handler.
func New(opts *Opts) *Handler {
	return &Handler{
		STSClient: opts.STSClient,
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
