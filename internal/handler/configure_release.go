package handler

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecr"
	common "github.com/mergermarket/cdflow2-config-common"
)

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {

	if err := h.InitReleaseAccountCredentials(request.Env, request.Team); err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	session, err := h.createReleaseAccountSession()
	if err != nil {
		return fmt.Errorf("unable to create AWS session in release account: %v", err)
	}

	releaseAccountCredentialsValue, err := h.ReleaseAccountCredentials.Get()
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	var ecrRepo string

	for buildID, reqs := range request.ReleaseRequirements {
		response.Env[buildID] = make(map[string]string)

		for _, need := range reqs.Needs {
			if need == "lambda" {
				response.Env[buildID]["LAMBDA_BUCKET"] = DefaultLambdaBucket
				setAWSEnvironmentVariables(response.Env[buildID], &releaseAccountCredentialsValue, Region)
			} else if need == "ecr" {
				if ecrRepo == "" {
					ecrRepo, err = h.getECRRepo(request.Component, session)
					if err != nil {
						fmt.Fprintln(h.ErrorStream, err)
						response.Success = false
						return nil
					}
				}
				response.Env[buildID]["ECR_REPOSITORY"] = ecrRepo
				setAWSEnvironmentVariables(response.Env[buildID], &releaseAccountCredentialsValue, Region)
			} else {
				fmt.Fprintf(h.ErrorStream, "unable to satisfy %q need for %q build", need, buildID)
				response.Success = false
				return nil
			}
		}

	}

	return nil
}

func setAWSEnvironmentVariables(env map[string]string, creds *credentials.Value, region string) {
	env["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
	env["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
	env["AWS_SESSION_TOKEN"] = creds.SessionToken
	env["AWS_DEFAULT_REGION"] = region
}

func (h *Handler) getECRRepo(componentName string, session client.ConfigProvider) (string, error) {
	ecrClient := h.ECRClientFactory(session)
	response, err := ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{aws.String(componentName)},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == ecr.ErrCodeRepositoryNotFoundException {
			return "", fmt.Errorf("no ecr repository found for %q", componentName)
		}
		return "", err
	}
	return *response.Repositories[0].RepositoryUri, nil
}
