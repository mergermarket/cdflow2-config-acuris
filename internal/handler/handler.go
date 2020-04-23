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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
)

const (
	AccountID     = "724178030834"
	LambdaBucket  = "acuris-lambdas"
	ReleaseBucket = "acuris-releases"
	TFStateBucket = "acuris-tfstate"
	ReleaseFolder = "/release"
	Region        = "eu-west-1"
)

// InitReleaseAccountCredentials initialises the release account credentials.
func (h *Handler) InitReleaseAccountCredentials(env map[string]string, team string) error {
	if env["AWS_ACCESS_KEY_ID"] == "" || env["AWS_SECRET_ACCESS_KEY"] == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not found in env")
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
		return fmt.Errorf("unable to create a new AWS session: %v", err)
	}

	h.ReleaseAccountCredentials = credentials.NewCredentials(
		h.AssumeRoleProviderFactory(session, fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", AccountID, team), "TODO"),
	)
	return nil
}

func (h *Handler) releaseAccountCredentials() *credentials.Credentials {
	return h.ReleaseAccountCredentials
}

func (h *Handler) createReleaseAccountSession() (client.ConfigProvider, error) {
	return session.NewSession(
		aws.NewConfig().
			WithCredentials(h.ReleaseAccountCredentials).
			WithRegion(Region),
	)
}

// ECRClientFactory is a function that returns an ECR client.
type ECRClientFactory func(client.ConfigProvider) ecriface.ECRAPI

// S3ClientFactory is a function that returns an S3 client.
type S3ClientFactory func(client.ConfigProvider) s3iface.S3API

// AssumeRoleProviderFactory is a function that returns an assume role provider.
type AssumeRoleProviderFactory func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider

// Handler handles config requests.
type Handler struct {
	AssumeRoleProviderFactory AssumeRoleProviderFactory
	ReleaseAccountCredentials *credentials.Credentials
	ErrorStream               io.Writer
	ReleaseFolder             string
	ECRClientFactory          ECRClientFactory
	S3ClientFactory           S3ClientFactory
}

// New returns a new handler.
func New() Handler {
	return Handler{
		ErrorStream:   os.Stderr,
		ReleaseFolder: ReleaseFolder,
		ECRClientFactory: func(session client.ConfigProvider) ecriface.ECRAPI {
			return ecr.New(session)
		},
		S3ClientFactory: func(session client.ConfigProvider) s3iface.S3API {
			return s3.New(session)
		},
		AssumeRoleProviderFactory: func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
			return &stscreds.AssumeRoleProvider{
				Client:          sts.New(session),
				RoleARN:         roleARN,
				RoleSessionName: roleSessionName,
				Duration:        stscreds.DefaultDuration,
			}
		},
	}
}

// WithErrorStream overrides the stream where errors are written.
func (h Handler) WithErrorStream(errorStream io.Writer) Handler {
	h.ErrorStream = errorStream
	return h
}

// WithAssumeRoleProviderFactory overrides the function used to create an assume role provider.
func (h Handler) WithAssumeRoleProviderFactory(factory AssumeRoleProviderFactory) Handler {
	h.AssumeRoleProviderFactory = factory
	return h
}

// WithECRClientFactory overrides the function used to create an ECR client.
func (h Handler) WithECRClientFactory(factory ECRClientFactory) Handler {
	h.ECRClientFactory = factory
	return h
}

// WithS3ClientFactory overrides the function used to create an ECR client.
func (h Handler) WithS3ClientFactory(factory S3ClientFactory) Handler {
	h.S3ClientFactory = factory
	return h
}

// WithReleaseFolder overrides the release folder.
func (h Handler) WithReleaseFolder(folder string) Handler {
	h.ReleaseFolder = folder
	return h
}

func releaseS3Key(team, component, version string) string {
	return fmt.Sprintf("%s/%s/%s-%s.zip", team, component, component, version)
}
