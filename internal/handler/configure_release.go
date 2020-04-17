package handler

import (
	"fmt"

	common "github.com/mergermarket/cdflow2-config-common"
)

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	AWSClient := h.AWSClientFactory(request.Env)
	credentials, err := AWSClient.STSAssumeRole(request.Team)
	if err != nil {
		fmt.Fprintln(h.ErrorStream, "Unable to assume role:", err)
		response.Success = false
		return nil
	}

	for buildID, reqs := range request.ReleaseRequirements {
		response.Env[buildID] = make(map[string]string)
		response.Env[buildID]["AWS_ACCESS_KEY_ID"] = *credentials.AccessKeyId
		response.Env[buildID]["AWS_SECRET_ACCESS_KEY"] = *credentials.SecretAccessKey
		response.Env[buildID]["AWS_SESSION_TOKEN"] = *credentials.SessionToken
		response.Env[buildID]["AWS_DEFAULT_REGION"] = Region

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
				repo, err := AWSClient.GetECRRepoURI(request.Component)
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
