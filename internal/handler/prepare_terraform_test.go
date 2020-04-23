package handler_test

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
	"testing"
)

func TestPrepareTerraform(t *testing.T) {
	// Given
	accessKeyID := "foo"
	secretAccessKey := "bar"
	sessionToken := "baz"
	request := common.CreatePrepareTerraformRequest()
	request.Env["AWS_ACCESS_KEY_ID"] = "root foo"
	request.Env["AWS_SECRET_ACCESS_KEY"] = "root bar"
	request.Team = "such-team"
	request.Component = "such-component"
	request.EnvName = "such-env"
	response := common.CreatePrepareTerraformResponse()

	mockAssumeRoleProviderFactory := func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
		return createMockAssumeRoleProvider(accessKeyID, secretAccessKey, sessionToken)
	}
	h := handler.New().
		WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory)

	// When
	h.PrepareTerraform(request, response)

	// Then
	if response.TerraformBackendConfig["access_key"] != accessKeyID {
		t.Fatalf("Want %q, got %q", accessKeyID, response.TerraformBackendConfig["access_key"])
	}
	if response.TerraformBackendConfig["secret_key"] != secretAccessKey {
		t.Fatalf("Want %q, got %q", secretAccessKey, response.TerraformBackendConfig["secret_key"])
	}
	if response.TerraformBackendConfig["token"] != sessionToken {
		t.Fatalf("Want %q, got %q", sessionToken, response.TerraformBackendConfig["token"])
	}
	expectedKey := "such-team/such-component/such-env/terraform.tfstate"
	if response.TerraformBackendConfig["key"] != expectedKey {
		t.Fatalf("Want %q, got %q", expectedKey, response.TerraformBackendConfig["key"])
	}
	expectedDynamoDBTable := "such-team-tflocks"
	if response.TerraformBackendConfig["dynamodb_table"] != expectedDynamoDBTable {
		t.Fatalf("Want %q, got %q", expectedDynamoDBTable, response.TerraformBackendConfig["dynamodb_table"])
	}
}
