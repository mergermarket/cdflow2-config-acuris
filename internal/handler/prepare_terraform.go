package handler

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	common "github.com/mergermarket/cdflow2-config-common"
)

// PrepareTerraform runs before terraform to configure.
func (h *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse) error {
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
	response.TerraformBackendConfig["key"] = fmt.Sprintf("%s/%s/%s/terraform.tfstate", request.Team, request.Component, request.EnvName)
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
	if err := common.UnzipRelease(getObjectOutput.Body, *getObjectOutput.ContentLength, h.ReleaseFolder, request.Component, request.Version); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, "error unzipping release:", err)
		return nil
	}

	return nil
}
