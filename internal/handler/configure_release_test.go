package handler_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"

	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type MockECRClient struct {
	ecriface.ECRAPI
}

func (*MockECRClient) DescribeRepositories(input *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
	return &ecr.DescribeRepositoriesOutput{
		Repositories: []*ecr.Repository{
			{RepositoryUri: aws.String("repo:" + *input.RepositoryNames[0])},
		},
	}, nil
}

type MockECRClientNoRepo struct {
	ecriface.ECRAPI
}

func (m MockECRClientNoRepo) DescribeRepositories(input *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
	return &ecr.DescribeRepositoriesOutput{}, awserr.New(ecr.ErrCodeRepositoryNotFoundException, "repository not found", nil)
}

func createConfigureReleaseRequest() *common.ConfigureReleaseRequest {
	request := common.CreateConfigureReleaseRequest()
	request.Env["AWS_ACCESS_KEY_ID"] = "foo"
	request.Env["AWS_SECRET_ACCESS_KEY"] = "bar"
	return request
}

func TestConfigureRelease(t *testing.T) {

	accessKeyID := "test-access-key-id"
	secretAccessKey := "test-secret-access-key"
	sessionToken := "test-session-token"
	mockAssumeRoleProviderFactory := func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
		return createMockAssumeRoleProvider(accessKeyID, secretAccessKey, sessionToken)
	}

	expectedEnvVars := map[string]string{
		"AWS_ACCESS_KEY_ID":     accessKeyID,
		"AWS_SECRET_ACCESS_KEY": secretAccessKey,
		"AWS_SESSION_TOKEN":     sessionToken,
		"AWS_DEFAULT_REGION":    handler.Region,
	}

	t.Run("Lambda build", func(t *testing.T) {
		// Given
		request := createConfigureReleaseRequest()
		request.ReleaseRequirements = map[string]*common.ReleaseRequirements{
			"my-lambda": {Needs: []string{"lambda"}},
			"my-x":      {},
		}
		response := common.CreateConfigureReleaseResponse()

		h := handler.New().WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory)

		// When
		h.ConfigureRelease(request, response)

		// Then
		if !response.Success {
			t.Fatal("unexpected failure")
		}
		if len(response.Env) != 2 {
			t.Fatalf("Expected 2 builds, got %d", len(response.Env))
		}
		bucketName := response.Env["my-lambda"]["LAMBDA_BUCKET"]
		if bucketName != handler.LambdaBucket {
			t.Fatalf("got %q, want %q", bucketName, handler.LambdaBucket)
		}
		bucketName = response.Env["my-x"]["LAMBDA_BUCKET"]
		if bucketName != "" {
			t.Fatalf("my-x should not have LAMBDA_BUCKET, but got %q", bucketName)
		}
		for name, value := range expectedEnvVars {
			if response.Env["my-lambda"][name] != value {
				t.Fatalf("got %q for %q, expected %q", response.Env["my-lambda"][name], name, value)
			}
		}
	})

	t.Run("ECR build", func(t *testing.T) {
		// Given
		request := createConfigureReleaseRequest()
		request.Component = "my-component"
		request.ReleaseRequirements = map[string]*common.ReleaseRequirements{
			"my-ecr": {Needs: []string{"ecr"}},
			"my-x":   {},
		}
		response := common.CreateConfigureReleaseResponse()

		h := handler.New().
			WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
			WithECRClientFactory(func(session client.ConfigProvider) ecriface.ECRAPI {
				return &MockECRClient{}
			})

		// When
		h.ConfigureRelease(request, response)

		// Then
		if !response.Success {
			t.Fatal("unexpected failure")
		}
		if len(response.Env) != 2 {
			t.Fatalf("Expected 2 builds, got %d", len(response.Env))
		}
		ecrRepository := response.Env["my-ecr"]["ECR_REPOSITORY"]
		expectedRepository := "repo:my-component"
		if ecrRepository != expectedRepository {
			t.Fatalf("got %q, want %q", ecrRepository, expectedRepository)
		}
		for name, value := range expectedEnvVars {
			if response.Env["my-ecr"][name] != value {
				t.Fatalf("got %q for %q, expected %q", response.Env["my-ecr"][name], name, value)
			}
		}
	})

	t.Run("ECR build for nonexistent repo", func(t *testing.T) {
		// Given
		request := createConfigureReleaseRequest()
		request.Component = "nonexistent-component"
		request.ReleaseRequirements = map[string]*common.ReleaseRequirements{
			"my-ecr": {Needs: []string{"ecr"}},
		}
		response := common.CreateConfigureReleaseResponse()

		var errorBuffer bytes.Buffer

		h := handler.New().
			WithErrorStream(&errorBuffer).
			WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
			WithECRClientFactory(func(session client.ConfigProvider) ecriface.ECRAPI {
				return &MockECRClientNoRepo{}
			})

		// When
		h.ConfigureRelease(request, response)

		// Then
		if response.Success {
			t.Fatal("unexpected success")
		}
		expectedMessage := fmt.Sprintf("no ecr repository found for %q\n", request.Component)
		if errorBuffer.String() != expectedMessage {
			t.Fatalf("expected %q, got %q", expectedMessage, errorBuffer.String())
		}
	})

	t.Run("unsupported need for a build", func(t *testing.T) {
		// Given
		request := createConfigureReleaseRequest()
		request.ReleaseRequirements = map[string]*common.ReleaseRequirements{
			"something": {Needs: []string{"unsupported"}},
		}
		response := common.CreateConfigureReleaseResponse()

		var errorBuffer bytes.Buffer

		h := handler.New().
			WithErrorStream(&errorBuffer).
			WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory)

		// When
		h.ConfigureRelease(request, response)

		// Then
		if response.Success {
			t.Fatal("unexpected success")
		}
		if errorBuffer.String() != "unable to satisfy \"unsupported\" need for \"something\" build" {
			t.Fatalf("wrong error?: %q", errorBuffer.String())
		}
	})
}
