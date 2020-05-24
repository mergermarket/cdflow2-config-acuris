package handler

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// PrepareTerraform runs before terraform to configure.
func (h *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse, releaseDir string) error {
	if err := h.InitReleaseAccountCredentials(request.Env, request.Team); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	releaseAccountCredentialsValue, err := h.ReleaseAccountCredentials.Get()
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
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
		return fmt.Errorf("unable to create AWS session in release account: %v", err)
	}

	s3Client := h.S3ClientFactory(session)
	getObjectOutput, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(ReleaseBucket),
		Key:    aws.String(releaseS3Key(request.Team, request.Component, request.Version)),
	})
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	if err := h.AddDeployAccountCredentialsValue(request, response.Env); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	terraformImage, err := h.ReleaseLoader.Load(getObjectOutput.Body, request.Component, request.Version, releaseDir)
	if err != nil {
		return err
	}
	response.TerraformImage = terraformImage

	return nil
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

	var accountName string
	if request.EnvName == "live" {
		accountName = accountPrefix + "prod"
	} else {
		accountName = accountPrefix + "dev"
	}

	input := &organizations.ListAccountsInput{}
	var accountID string
	if err := orgsClient.ListAccountsPages(input, func(result *organizations.ListAccountsOutput, lastPage bool) bool {
		for _, account := range result.Accounts {
			if *account.Name == accountName {
				accountID = *account.Id
				return false
			}
		}
		return true
	}); err != nil {
		return err
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
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", accountID, request.Team)),
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
