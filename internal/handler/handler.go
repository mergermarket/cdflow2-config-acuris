package handler

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/organizations/organizationsiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

const (
	AccountID     = "724178030834"
	LambdaBucket  = "acuris-lambdas"
	ReleaseBucket = "acuris-releases"
	TFStateBucket = "acuris-tfstate"
	ReleaseFolder = "/release"
	Region        = "eu-west-1"
)

// GetRootAccountSession gets an AWS session in the root account.
func (h *Handler) GetRootAccountSession(env map[string]string) (*session.Session, error) {
	if h.RootAccountSession != nil {
		return h.RootAccountSession, nil
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
	h.RootAccountSession = session
	return session, nil
}

// InitReleaseAccountCredentials initialises the release account credentials.
func (h *Handler) InitReleaseAccountCredentials(env map[string]string, team string) error {
	session, err := h.GetRootAccountSession(env)
	if err != nil {
		return err
	}
	roleSessionName, err := GetRoleSessionName(env)
	if err != nil {
		return err
	}

	h.ReleaseAccountCredentials = credentials.NewCredentials(
		h.AssumeRoleProviderFactory(session, fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", AccountID, team), roleSessionName),
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

// S3UploaderFactory is a function that returns an s3manager uploader.
type S3UploaderFactory func(client.ConfigProvider) s3manageriface.UploaderAPI

// AssumeRoleProviderFactory is a function that returns an assume role provider.
type AssumeRoleProviderFactory func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider

// STSClientFactory is a function that returns an STS client.
type STSClientFactory func(client.ConfigProvider) stsiface.STSAPI

// OrganizationsClientFactory is a function that returns an organizations client.
type OrganizationsClientFactory func(client.ConfigProvider) organizationsiface.OrganizationsAPI

// Handler handles config requests.
type Handler struct {
	RootAccountSession         *session.Session
	AssumeRoleProviderFactory  AssumeRoleProviderFactory
	ReleaseAccountCredentials  *credentials.Credentials
	ErrorStream                io.Writer
	ReleaseFolder              string
	ECRClientFactory           ECRClientFactory
	S3ClientFactory            S3ClientFactory
	S3UploaderFactory          S3UploaderFactory
	STSClientFactory           STSClientFactory
	OrganizationsClientFactory OrganizationsClientFactory
}

// New returns a new handler.
func New() *Handler {
	return &Handler{
		ErrorStream:   os.Stderr,
		ReleaseFolder: ReleaseFolder,
		OrganizationsClientFactory: func(session client.ConfigProvider) organizationsiface.OrganizationsAPI {
			return organizations.New(session)
		},
		ECRClientFactory: func(session client.ConfigProvider) ecriface.ECRAPI {
			return ecr.New(session)
		},
		S3ClientFactory: func(session client.ConfigProvider) s3iface.S3API {
			return s3.New(session)
		},
		S3UploaderFactory: func(session client.ConfigProvider) s3manageriface.UploaderAPI {
			return s3manager.NewUploaderWithClient(s3.New(session))
		},
		STSClientFactory: func(session client.ConfigProvider) stsiface.STSAPI {
			return sts.New(session)
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
func (h *Handler) WithErrorStream(errorStream io.Writer) *Handler {
	h.ErrorStream = errorStream
	return h
}

// WithAssumeRoleProviderFactory overrides the function used to create an assume role provider.
func (h *Handler) WithAssumeRoleProviderFactory(factory AssumeRoleProviderFactory) *Handler {
	h.AssumeRoleProviderFactory = factory
	return h
}

// WithECRClientFactory overrides the function used to create an ECR client.
func (h *Handler) WithECRClientFactory(factory ECRClientFactory) *Handler {
	h.ECRClientFactory = factory
	return h
}

// WithS3ClientFactory overrides the function used to create an S3 client.
func (h *Handler) WithS3ClientFactory(factory S3ClientFactory) *Handler {
	h.S3ClientFactory = factory
	return h
}

// WithS3UploaderFactory overrides the function used to create an S3 uploader.
func (h *Handler) WithS3UploaderFactory(factory S3UploaderFactory) *Handler {
	h.S3UploaderFactory = factory
	return h
}

// WithSTSClientFactory overrides the function used to create an STS client.
func (h *Handler) WithSTSClientFactory(factory STSClientFactory) *Handler {
	h.STSClientFactory = factory
	return h
}

// WithOrganizationsClientFactory overrides the function used to create an Organizations client.
func (h *Handler) WithOrganizationsClientFactory(factory OrganizationsClientFactory) *Handler {
	h.OrganizationsClientFactory = factory
	return h
}

// WithReleaseFolder overrides the release folder.
func (h *Handler) WithReleaseFolder(folder string) *Handler {
	h.ReleaseFolder = folder
	return h
}

func releaseS3Key(team, component, version string) string {
	return fmt.Sprintf("%s/%s/%s-%s.zip", team, component, component, version)
}

var sessionNameStripper *regexp.Regexp = regexp.MustCompile("[^\\w+=,.@-]")

// GetRoleSessionName returns a suitable role session name from the environment.
func GetRoleSessionName(env map[string]string) (string, error) {
	candidates := []string{
		"ROLE_SESSION_NAME",
		"JOB_NAME",
		"EMAIL",
	}
	var value string
	var name string
	for _, candidate := range candidates {
		if env[candidate] != "" {
			value = env[candidate]
			name = candidate
			break
		}
	}
	if value == "" {
		return "", fmt.Errorf("error - no role session name, please set one of these as an environment variable: %s", strings.Join(candidates, ", "))
	}
	value = sessionNameStripper.ReplaceAllLiteralString(value, "")
	if len(value) < 2 {
		return "", fmt.Errorf("role session name variable %q does not have enough valid characters (must have at least two)", name)
	}
	if len(value) > 64 {
		return value[:64], nil
	}
	return value, nil
}
