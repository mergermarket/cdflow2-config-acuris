package handler

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/sts"
	common "github.com/mergermarket/cdflow2-config-common"
)

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	STSClient, err := h.STSClientFactory(request.Env)
	if err != nil {
		fmt.Fprintln(h.ErrorStream, err)
		response.Success = false
		return nil
	}
	result, err := STSClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", AccountID, request.Team)),
		RoleSessionName: aws.String("role-session-name"),
	})
	if err != nil {
		fmt.Fprintln(h.ErrorStream, "Unable to assume role:", err)
		response.Success = false
		return nil
	}

	ecrClient, err := h.ECRClientFactory(request.Env)
	if err != nil {
		fmt.Fprintln(h.ErrorStream, err)
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
			} else if need == "ecr" {
				repo, err := h.getEcrRepo(request.Component, ecrClient)
				if err != nil {
					response.Success = false
					fmt.Fprintln(h.ErrorStream, err)
					return nil
				}
				response.Env[buildID]["ECR_REPOSITORY"] = repo
			} else {
				fmt.Fprintf(h.ErrorStream, "unable to satisfy %q need for %q build", need, buildID)
				response.Success = false
				return nil
			}
		}
	}

	response.Success = true
	return nil
}

func (h *Handler) getEcrRepo(componentName string, ecrClient ecriface.ECRAPI) (string, error) {
	response, err := ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{aws.String(componentName)},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == ecr.ErrCodeRepositoryNotFoundException {
			return "", fmt.Errorf("ECR repository for %s does not exist", componentName)
		} else {
			return "", err
		}
	}
	return *response.Repositories[0].RepositoryUri, nil
}
