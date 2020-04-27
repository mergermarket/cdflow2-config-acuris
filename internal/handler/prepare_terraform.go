package handler

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// PrepareTerraform runs before terraform to configure.
func (h *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse) (io.Reader, error) {
	if err := h.InitReleaseAccountCredentials(request.Env, request.Team); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil, nil
	}

	releaseAccountCredentialsValue, err := h.ReleaseAccountCredentials.Get()
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil, nil
	}

	response.TerraformBackendType = "s3"
	response.TerraformBackendConfig["access_key"] = releaseAccountCredentialsValue.AccessKeyID
	response.TerraformBackendConfig["secret_key"] = releaseAccountCredentialsValue.SecretAccessKey
	response.TerraformBackendConfig["token"] = releaseAccountCredentialsValue.SessionToken
	response.TerraformBackendConfig["region"] = Region
	response.TerraformBackendConfig["bucket"] = TFStateBucket
	response.TerraformBackendConfig["workspace_key_prefix"] = fmt.Sprintf("%s/%s", request.Team, request.Component)
	response.TerraformBackendConfig["key"] = "terraform.tfstate"
	response.TerraformBackendConfig["dynamodb_table"] = fmt.Sprintf("%s-tflocks", request.Team)

	session, err := h.createReleaseAccountSession()
	if err != nil {
		return nil, fmt.Errorf("unable to create AWS session in release account: %v", err)
	}

	s3Client := h.S3ClientFactory(session)
	getObjectOutput, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(ReleaseBucket),
		Key:    aws.String(releaseS3Key(request.Team, request.Component, request.Version)),
	})
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil, nil
	}

	if err := h.AddDeployAccountCredentialsValue(request, response.Env); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil, nil
	}

	return getObjectOutput.Body, nil
}

// AddDeployAccountCredentialsValue assumes a role in the right account and returns credentials.
func (h *Handler) AddDeployAccountCredentialsValue(request *common.PrepareTerraformRequest, responseEnv map[string]string) error {
	accountPrefix, ok := request.Config["accountprefix"].(string)
	if !ok || accountPrefix == "" {
		return fmt.Errorf("config.params.accountprefix must be set and be a string value")
	}
	session, err := h.GetRootAccountSession(request.Env)
	if err != nil {
		return err
	}

	orgsClient := h.OrganizationsClientFactory(session)
	accounts, err := orgsClient.ListAccounts(&organizations.ListAccountsInput{})
	if err != nil {
		return err
	}
	var accountName string
	if request.EnvName == "live" {
		accountName = accountPrefix + "prod"
	} else {
		accountName = accountPrefix + "dev"
	}
	var accountID string
	for _, account := range accounts.Accounts {
		if *account.Name == accountName {
			accountID = *account.Id
			break
		}
	}
	if accountID == "" {
		return fmt.Errorf("account %q not found", accountName)
	}

	roleSessionName, err := GetRoleSessionName(request.Env)
	if err != nil {
		return err
	}

	stsClient := h.STSClientFactory(session)
	result, err := stsClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/deploy-%s", accountID, request.Team)),
		RoleSessionName: aws.String(roleSessionName),
	})
	if err != nil {
		return err
	}

	responseEnv["AWS_ACCESS_KEY_ID"] = *result.Credentials.AccessKeyId
	responseEnv["AWS_SECRET_ACCESS_KEY"] = *result.Credentials.SecretAccessKey
	responseEnv["AWS_SESSION_TOKEN"] = *result.Credentials.SessionToken
	responseEnv["AWS_DEFAULT_REGION"] = Region

	return nil
}
