package handler_test

import (
	"github.com/aws/aws-sdk-go/service/sts"
	"testing"

	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type mockedSTS struct {
	stsiface.STSAPI
	envVars map[string]string
}

func (s *mockedSTS) AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	id := s.envVars["accessKeyId"]
	key := s.envVars["secretAccessKey"]
	token := s.envVars["sessionToken"]
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     &id,
			SecretAccessKey: &key,
			SessionToken:    &token,
		},
	}, nil
}

func TestConfigureRelease(t *testing.T) {
	// Given
	request := common.CreateConfigureReleaseRequest()
	request.Team = "my-team"
	request.ReleaseRequiredEnv = map[string][]string{
		"service1": {},
		"service2": {},
	}
	response := common.CreateConfigureReleaseResponse()

	envVars := make(map[string]string)
	envVars["accessKeyId"] = "AccessKeyId"
	envVars["secretAccessKey"] = "SecretAccessKey"
	envVars["sessionToken"] = "SessionToken"
	handler := handler.New(&handler.Opts{
		STSClient: &mockedSTS{
			envVars: envVars,
		},
	})

	// When
	err := handler.ConfigureRelease(request, response)

	// Then
	if err != nil {
		t.Fatalf("Did not expect an error, but got one: %v", err)
	}
	// @todo is response.Success actually being used elsewhere? Shouldn't err returned from the method be enough?
	if !response.Success {
		t.Fatal("unexpected failure")
	}
	if len(response.Env) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(response.Env))
	}
	for id := range response.Env {
		if id != "service1" && id != "service2" {
			t.Fatalf("Unexpected service %q", id)
		}
		for _, envVarName := range []string{
			"AWS_ACCESS_KEY_ID",
			"AWS_SECRET_ACCESS_KEY",
			"AWS_SESSION_TOKEN",
			"AWS_DEFAULT_REGION",
		} {
			if response.Env[id][envVarName] == "" {
				t.Fatalf("%q is empty", envVarName)
			}
		}
	}
}
