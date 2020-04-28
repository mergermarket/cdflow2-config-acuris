package handler_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/organizations/organizationsiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type MockSTSClient struct {
	stsiface.STSAPI
	accessKeyID            string
	secretAccessKey        string
	sessionToken           string
	assumedRoleArn         string
	assumedRoleSessionName string
}

func (m *MockSTSClient) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	if m.assumedRoleArn != "" {
		panic("assume role already called")
	}
	m.assumedRoleArn = *input.RoleArn
	m.assumedRoleSessionName = *input.RoleSessionName
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String(m.accessKeyID),
			SecretAccessKey: aws.String(m.secretAccessKey),
			SessionToken:    aws.String(m.sessionToken),
		},
	}, nil
}

type MockOrganizationsClient struct {
	organizationsiface.OrganizationsAPI
	Accounts map[string]string
}

func (m *MockOrganizationsClient) ListAccountsPages(input *organizations.ListAccountsInput, callback func(*organizations.ListAccountsOutput, bool) bool) error {
	output := organizations.ListAccountsOutput{}
	for name, id := range m.Accounts {
		output.Accounts = append(output.Accounts, &organizations.Account{
			Id:     aws.String(id),
			Name:   aws.String(name),
			Status: aws.String("ACTIVE"),
		})
	}
	callback(&output, false)
	return nil
}

func TestPrepareTerraform(t *testing.T) {
	// Given

	request := common.CreatePrepareTerraformRequest()
	request.Env["AWS_ACCESS_KEY_ID"] = "root foo"
	request.Env["AWS_SECRET_ACCESS_KEY"] = "root bar"
	request.Env["ROLE_SESSION_NAME"] = "baz"
	request.Team = "such-team"
	request.Component = "such-component"
	request.EnvName = "live"
	request.Config["accountprefix"] = "foo"
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

	deployAccessKeyID := "do"
	deploySecretAccessKey := "re"
	deploySessionToken := "mi"
	mockSTSClient := &MockSTSClient{
		accessKeyID:     deployAccessKeyID,
		secretAccessKey: deploySecretAccessKey,
		sessionToken:    deploySessionToken,
	}

	deployAccountID := "1234567890"
	mockOrganizationsClient := &MockOrganizationsClient{
		Accounts: map[string]string{
			"foodev":  "0987654321",
			"fooprod": deployAccountID,
			"bardev":  "00000000000",
			"barprod": "11111111111",
			"other":   "22222222222",
		},
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
		}).
		WithSTSClientFactory(func(client.ConfigProvider) stsiface.STSAPI {
			return mockSTSClient
		}).
		WithOrganizationsClientFactory(func(client.ConfigProvider) organizationsiface.OrganizationsAPI {
			return mockOrganizationsClient
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

	if response.Env["AWS_ACCESS_KEY_ID"] != deployAccessKeyID {
		t.Fatalf("Want %q, got %q", deployAccessKeyID, response.Env["AWS_ACCESS_KEY_ID"])
	}
	if response.Env["AWS_SECRET_ACCESS_KEY"] != deploySecretAccessKey {
		t.Fatalf("Want %q, got %q", deploySecretAccessKey, response.Env["AWS_SECRET_ACCESS_KEY"])
	}
	if response.Env["AWS_SESSION_TOKEN"] != deploySessionToken {
		t.Fatalf("Want %q, got %q", deploySessionToken, response.Env["AWS_SESSION_TOKEN"])
	}
	if response.Env["AWS_DEFAULT_REGION"] != handler.Region {
		t.Fatalf("Want %q, got %q", handler.Region, response.Env["AWS_DEFAULT_REGION"])
	}
	expectedDeployRole := fmt.Sprintf("arn:aws:iam::%s:role/such-team-deploy", deployAccountID)
	if mockSTSClient.assumedRoleArn != expectedDeployRole {
		t.Fatalf("Want %q, got %q", expectedDeployRole, mockSTSClient.assumedRoleArn)
	}
	expectedDeploySessionName := "baz"
	if mockSTSClient.assumedRoleSessionName != expectedDeploySessionName {
		t.Fatalf("Want %q, got %q", expectedDeploySessionName, mockSTSClient.assumedRoleSessionName)
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
