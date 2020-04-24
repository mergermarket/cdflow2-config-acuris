package handler_test

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

func TestPrepareTerraform(t *testing.T) {
	// Given

	request := common.CreatePrepareTerraformRequest()
	request.Env["AWS_ACCESS_KEY_ID"] = "root foo"
	request.Env["AWS_SECRET_ACCESS_KEY"] = "root bar"
	request.Team = "such-team"
	request.Component = "such-component"
	request.EnvName = "such-env"
	response := common.CreatePrepareTerraformResponse()

	accessKeyID := "foo"
	secretAccessKey := "bar"
	sessionToken := "baz"

	file, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	mockS3Client := &MockS3Client{
		getObjectBody: file,
	}
	mockAssumeRoleProviderFactory := func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
		return createMockAssumeRoleProvider(accessKeyID, secretAccessKey, sessionToken)
	}

	releaseFolder, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(releaseFolder)

	h := handler.New().
		WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
		WithS3ClientFactory(func(client.ConfigProvider) s3iface.S3API {
			return mockS3Client
		})

	// When
	reader, err := h.PrepareTerraform(request, response)
	if err != nil {
		t.Fatal(err)
	}

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
	expectedKey := "terraform.tfstate"
	if response.TerraformBackendConfig["key"] != expectedKey {
		t.Fatalf("Want %q, got %q", expectedKey, response.TerraformBackendConfig["key"])
	}
	expectedWorkspaceKeyPrefix := "such-team/such-component"
	if response.TerraformBackendConfig["workspace_key_prefix"] != expectedWorkspaceKeyPrefix {
		t.Fatalf("Want %q, got %q", expectedWorkspaceKeyPrefix, response.TerraformBackendConfig["workspace_key_prefix"])
	}
	expectedDynamoDBTable := "such-team-tflocks"
	if response.TerraformBackendConfig["dynamodb_table"] != expectedDynamoDBTable {
		t.Fatalf("Want %q, got %q", expectedDynamoDBTable, response.TerraformBackendConfig["dynamodb_table"])
	}
	if file != reader {
		t.Fatalf("got %v, wanted %v", reader, file)
	}
}
