package handler

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// ConfigureRelease runs before release to configure it.
func (handler *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	STSClient := sts.New(session.New())
	result, err := STSClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::724178030834:role/%s-deploy", request.Team)),
		RoleSessionName: aws.String("role-session-name"),
	})
	if err != nil {
		return err
	}

	for buildID := range request.ReleaseRequiredEnv {
		response.Env[buildID] = make(map[string]string)
		response.Env[buildID]["AWS_ACCESS_KEY_ID"] = *result.Credentials.AccessKeyId
		response.Env[buildID]["AWS_SECRET_ACCESS_KEY"] = *result.Credentials.SecretAccessKey
		response.Env[buildID]["AWS_SESSION_TOKEN"] = *result.Credentials.SessionToken
		response.Env[buildID]["AWS_DEFAULT_REGION"] = "eu-west-1"
	}

	return nil
}
