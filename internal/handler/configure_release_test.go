package handler_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type mockedSTS struct {
	stsiface.STSAPI
	creds map[string]string
}

func (s *mockedSTS) AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String(s.creds["accessKeyId"]),
			SecretAccessKey: aws.String(s.creds["secretAccessKey"]),
			SessionToken:    aws.String(s.creds["sessionToken"]),
		},
	}, nil
}

func TestConfigureRelease(t *testing.T) {
	// Given
	request := common.CreateConfigureReleaseRequest()
	request.Team = "my-team"
	request.ReleaseRequiredEnv = map[string][]string{
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
			return &mockedSTS{
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
}
