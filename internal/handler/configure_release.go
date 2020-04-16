package handler

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

const DefaultLambdaBucket = "acuris-lambdas"

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	STSClient, err := h.STSClientFactory(request.Env)
	if err != nil {
		fmt.Fprintln(h.ErrorStream, err)
		response.Success = false
		return nil
	}
	result, err := STSClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::724178030834:role/%s-deploy", request.Team)),
		RoleSessionName: aws.String("role-session-name"),
	})
	if err != nil {
		fmt.Fprintln(h.ErrorStream, "Unable to assume role:", err)
		response.Success = false
		return nil
	}

	for buildID, reqs := range request.ReleaseRequirements {
		response.Env[buildID] = make(map[string]string)
		response.Env[buildID]["AWS_ACCESS_KEY_ID"] = *result.Credentials.AccessKeyId
		response.Env[buildID]["AWS_SECRET_ACCESS_KEY"] = *result.Credentials.SecretAccessKey
		response.Env[buildID]["AWS_SESSION_TOKEN"] = *result.Credentials.SessionToken
		response.Env[buildID]["AWS_DEFAULT_REGION"] = "eu-west-1"

		needs, ok := reqs["needs"]
		if !ok {
			continue
		}
		listOfNeeds, ok := needs.([]string)
		if !ok {
			return fmt.Errorf("unexpected type of _needs_ from %q, expected []string, got %T", buildID, reqs["needs"])
		}
		for _, need := range listOfNeeds {
			if need == "lambda" {
				response.Env[buildID]["LAMBDA_BUCKET"] = DefaultLambdaBucket
			}
		}
	}

	response.Success = true
	return nil
}
