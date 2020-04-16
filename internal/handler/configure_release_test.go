package handler_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type mockedSTSClient struct {
	stsiface.STSAPI
	creds map[string]string
}

func (c *mockedSTSClient) AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String(c.creds["accessKeyId"]),
			SecretAccessKey: aws.String(c.creds["secretAccessKey"]),
			SessionToken:    aws.String(c.creds["sessionToken"]),
		},
	}, nil
}

type failingSTSClient struct {
	stsiface.STSAPI
	errorText string
}

func (c *failingSTSClient) AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return &sts.AssumeRoleOutput{}, fmt.Errorf(c.errorText)
}

func TestConfigureRelease(t *testing.T) {
	t.Run("aws creds happy path", func(t *testing.T) {
		// Given
		request := common.CreateConfigureReleaseRequest()
		request.Team = "my-team"
		request.ReleaseRequirements = map[string]map[string]interface{}{
			"build1": {},
			"build2": {},
		}
		response := common.CreateConfigureReleaseResponse()

		assumeRoleCreds := make(map[string]string)
		assumeRoleCreds["accessKeyId"] = "AccessKeyId"
		assumeRoleCreds["secretAccessKey"] = "SecretAccessKey"
		assumeRoleCreds["sessionToken"] = "SessionToken"
		handler := handler.New(&handler.Opts{
			STSClientFactory: func(map[string]string) (stsiface.STSAPI, error) {
				return &mockedSTSClient{
					creds: assumeRoleCreds,
				}, nil
			},
		})

		// When
		handler.ConfigureRelease(request, response)

		// Then
		if !response.Success {
			t.Fatal("unexpected failure")
		}
		if len(response.Env) != 2 {
			t.Fatalf("Expected 2 builds, got %d", len(response.Env))
		}
		expectedEnvVars := map[string]string{
			"AWS_ACCESS_KEY_ID":     assumeRoleCreds["accessKeyId"],
			"AWS_DEFAULT_REGION":    "eu-west-1",
			"AWS_SECRET_ACCESS_KEY": assumeRoleCreds["secretAccessKey"],
			"AWS_SESSION_TOKEN":     assumeRoleCreds["sessionToken"],
		}
		for id := range response.Env {
			if id != "build1" && id != "build2" {
				t.Fatalf("Unexpected build env %q", id)
			}
			if !reflect.DeepEqual(response.Env[id], expectedEnvVars) {
				t.Fatalf("Expected %+v, got %+v", expectedEnvVars, response.Env[id])
			}
		}
	})

	t.Run("aws credentials failing client factory", func(t *testing.T) {
		// Given
		var errorBuffer bytes.Buffer

		request := common.CreateConfigureReleaseRequest()
		response := common.CreateConfigureReleaseResponse()

		errorText := "test-error-text"
		handler := handler.New(&handler.Opts{
			STSClientFactory: func(map[string]string) (stsiface.STSAPI, error) {
				return nil, fmt.Errorf(errorText)
			},
			ErrorStream: &errorBuffer,
		})

		// When
		handler.ConfigureRelease(request, response)

		// Then
		if response.Success {
			t.Fatal("unexpected success")
		}
		if errorBuffer.String() != errorText+"\n" {
			t.Fatalf("expected %q, got %q", errorText+"\n", errorBuffer.String())
		}
	})

	t.Run("aws credentials failing client to assume role", func(t *testing.T) {
		// Given
		var errorBuffer bytes.Buffer

		request := common.CreateConfigureReleaseRequest()
		response := common.CreateConfigureReleaseResponse()

		errorText := "test-error-text"
		handler := handler.New(&handler.Opts{
			STSClientFactory: func(map[string]string) (stsiface.STSAPI, error) {
				return &failingSTSClient{errorText: errorText}, nil
			},
			ErrorStream: &errorBuffer,
		})

		// When
		handler.ConfigureRelease(request, response)

		// Then
		if response.Success {
			t.Fatal("unexpected success")
		}
		fullMessage := "Unable to assume role: " + errorText + "\n"
		if errorBuffer.String() != fullMessage {
			t.Fatalf("expected %q, got %q", fullMessage, errorBuffer.String())
		}
	})
}
