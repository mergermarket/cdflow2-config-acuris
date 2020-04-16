package handler

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	STSClient, err := h.STSClientFactory(request.Env)
	if err != nil {
		// @todo change this to use output from the handler
		fmt.Fprintln(os.Stderr, err)
		response.Success = false
		return nil
	}
	result, err := STSClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::724178030834:role/%s-deploy", request.Team)),
		RoleSessionName: aws.String("role-session-name"),
	})
	if err != nil {
		// @todo change this to use output from the handler
		fmt.Fprintf(os.Stderr, "Unable to assume role: %v", err)
		response.Success = false
		return nil
	}

	for buildID := range request.ReleaseRequiredEnv {
		response.Env[buildID] = make(map[string]string)
		response.Env[buildID]["AWS_ACCESS_KEY_ID"] = *result.Credentials.AccessKeyId
		response.Env[buildID]["AWS_SECRET_ACCESS_KEY"] = *result.Credentials.SecretAccessKey
		response.Env[buildID]["AWS_SESSION_TOKEN"] = *result.Credentials.SessionToken
		response.Env[buildID]["AWS_DEFAULT_REGION"] = "eu-west-1"
	}

	response.Success = true
	return nil
}
