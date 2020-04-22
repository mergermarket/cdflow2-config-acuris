package handler

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

const AccountID = "724178030834"
const Region = "eu-west-1"
const DefaultLambdaBucket = "acuris-lambdas"

func (h *Handler) getAssumeRoleProvider(c client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
	if h.OverrideAssumeRoleProvider != nil {
		return h.OverrideAssumeRoleProvider
	}
	return &stscreds.AssumeRoleProvider{
		Client:          sts.New(c),
		RoleARN:         roleARN,
		RoleSessionName: roleSessionName,
		Duration:        stscreds.DefaultDuration,
	}
}

func (h *Handler) getReleaseAccountCredentials(env map[string]string, team string) (*credentials.Credentials, error) {
	if h.ReleaseAccountCredentials != nil {
		return h.ReleaseAccountCredentials, nil
	}
	if env["AWS_ACCESS_KEY_ID"] == "" || env["AWS_SECRET_ACCESS_KEY"] == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not found in env")
	}
	config := aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(
			env["AWS_ACCESS_KEY_ID"],
			env["AWS_SECRET_ACCESS_KEY"],
			env["AWS_SESSION_TOKEN"],
		)).
		WithRegion(Region)

	session, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create a new AWS session: %v", err)
	}

	h.ReleaseAccountCredentials = credentials.NewCredentials(
		h.getAssumeRoleProvider(session, fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", AccountID, team), "TODO"),
	)
	return h.ReleaseAccountCredentials, nil
}

// ECRClientFactory is a function that returns an ECR client.
type ECRClientFactory func(client.ConfigProvider) ecriface.ECRAPI

// Handler handles config requests.
type Handler struct {
	OverrideAssumeRoleProvider credentials.Provider
	ReleaseAccountCredentials  *credentials.Credentials
	ErrorStream                io.Writer
	ECRClientFactory           ECRClientFactory
}

// New returns a new handler.
func New() *Handler {
	return &Handler{
		ErrorStream: os.Stderr,
		ECRClientFactory: func(session client.ConfigProvider) ecriface.ECRAPI {
			return ecr.New(session)
		},
	}
}

// WithErrorStream overrides the stream where errors are written.
func (h *Handler) WithErrorStream(errorStream io.Writer) *Handler {
	h.ErrorStream = errorStream
	return h
}

// WithAssumeRoleProvider overrides the credentials provider used to assume roles.
func (h *Handler) WithAssumeRoleProvider(provider credentials.Provider) *Handler {
	h.OverrideAssumeRoleProvider = provider
	return h
}

// WithECRClientFactory overrides the function used to create an ECR client.
func (h *Handler) WithECRClientFactory(factory ECRClientFactory) *Handler {
	h.ECRClientFactory = factory
	return h
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
