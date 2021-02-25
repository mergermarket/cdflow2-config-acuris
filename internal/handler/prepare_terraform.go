package handler

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// PrepareTerraform runs before terraform to configure.
func (h *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse, releaseDir string) error {
	team, err := h.getTeam(request.Config["team"])
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	if err := h.InitReleaseAccountCredentials(request.Env, team); err != nil {
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
	response.TerraformBackendConfig["workspace_key_prefix"] = fmt.Sprintf("%s/%s", team, request.Component)
	response.TerraformBackendConfig["key"] = "terraform.tfstate"
	response.TerraformBackendConfig["dynamodb_table"] = fmt.Sprintf("%s-tflocks", team)

	session, err := h.createReleaseAccountSession()
	if err != nil {
		return fmt.Errorf("unable to create AWS session in release account: %v", err)
	}

	if err := h.AddDeployAccountCredentialsValue(request, team, response.Env); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	AddAdditionalEnvironment(request.Env, response.Env)

	if request.Version == "" {
		return nil
	}

	s3Client := h.S3ClientFactory(session)

	key := releaseS3Key(team, request.Component, request.Version)
	fmt.Fprintf(h.ErrorStream, "- Downloading release from s3://%s/%s...\n", ReleaseBucket, key)

	getObjectOutput, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(ReleaseBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	terraformImage, err := h.ReleaseLoader.Load(
		getObjectOutput.Body, request.Component, request.Version, releaseDir,
		func(path, checksum string) (io.ReadCloser, error) {
			expectedPrefix := ".terraform/plugins/"
			if !strings.HasPrefix(path, expectedPrefix) {
				return nil, fmt.Errorf("expected path %q to start with %q", path, expectedPrefix)
			}
			name := path[len(expectedPrefix):]
			reader, err := os.Open("/cache/terraform-plugin-cache/" + name)
			if err == nil {
				return reader, nil
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			fmt.Fprintf(h.ErrorStream, "- Downloading provider plugin %s...\n", name)
			getObjectOutput, err := s3Client.GetObject(&s3.GetObjectInput{
				Bucket: aws.String(ReleaseBucket),
				Key:    aws.String(savedPluginKey(team, path, checksum)),
			})
			if err != nil {
				return nil, err
			}
			return getObjectOutput.Body, nil
		},
	)
	if err != nil {
		return err
	}
	response.TerraformImage = terraformImage

	return nil
}

func (h *Handler) addRootAccountCredentials(requestEnv map[string]string, responseEnv map[string]string) error {
	if requestEnv["AWS_ACCESS_KEY_ID"] == "" || requestEnv["AWS_SECRET_ACCESS_KEY"] == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not found in env")
	}
	responseEnv["AWS_ACCESS_KEY_ID"] = requestEnv["AWS_ACCESS_KEY_ID"]
	responseEnv["AWS_SECRET_ACCESS_KEY"] = requestEnv["AWS_SECRET_ACCESS_KEY"]
	responseEnv["AWS_SESSION_TOKEN"] = requestEnv["AWS_SESSION_TOKEN"]
	responseEnv["AWS_DEFAULT_REGION"] = Region
	return nil
}

// Contains takes a slice and looks for val as an element in it. If found it will
// return true, otherwise it will return a false.
func contains(val string, slice []string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// AddDeployAccountCredentialsValue assumes a role in the right account and returns credentials.
func (h *Handler) AddDeployAccountCredentialsValue(request *common.PrepareTerraformRequest, team string, responseEnv map[string]string) error {
	assumeRoleToDeploy, ok := request.Config["assume_role_to_deploy"].(bool)
	if ok && !assumeRoleToDeploy {
		return h.addRootAccountCredentials(request.Env, responseEnv)
	}

	accountPrefix, ok := request.Config["account_prefix"].(string)
	if !ok || accountPrefix == "" {
		return fmt.Errorf("cdflow.yaml: error - config.params.account_prefix must be set and be a string value")
	}

	prodEnvs := []string{"live"}
	additionalProdEnvsInterface, ok := request.Config["additional_prod_envs"].([]interface{})
	if ok {
		prodEnvs = make([]string, len(additionalProdEnvsInterface)+1)
		prodEnvs[0] = "live"
		for i, v := range additionalProdEnvsInterface {
			prodEnvs[i+1] = v.(string)
		}

		fmt.Fprintf(h.ErrorStream, "Found additional_prod_envs, appending them to the default resulting in: %v\n", prodEnvs)
	}

	var accountName string
	if contains(request.EnvName, prodEnvs) {
		accountName = accountPrefix + "prod"
	} else {
		accountName = accountPrefix + "dev"
	}

	role := team + "-deploy"

	fmt.Fprintf(h.ErrorStream, "- Assuming %q role in %q account...\n", role, accountName)

	session, err := h.GetRootAccountSession(request.Env)
	if err != nil {
		return err
	}

	orgsClient := h.OrganizationsClientFactory(session)

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
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, role)),
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

// AddAdditionalEnvironment variables sends in env variables
func AddAdditionalEnvironment(requestEnv map[string]string, responseEnv map[string]string) {
	responseEnv["DD_APP_KEY"] = requestEnv["DATADOG_APP_KEY"]
	responseEnv["DD_API_KEY"] = requestEnv["DATADOG_API_KEY"]
	responseEnv["FASTLY_API_KEY"] = requestEnv["FASTLY_API_KEY"]
	responseEnv["GITHUB_TOKEN"] = requestEnv["GITHUB_TOKEN"]
}
