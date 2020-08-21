package handler

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	common "github.com/mergermarket/cdflow2-config-common"
)

const ECR_REPO_POLICY = `
{
	"Version": "2008-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": "*",
			"Action": [
				"ecr:BatchCheckLayerAvailability",
				"ecr:BatchGetImage",
				"ecr:GetDownloadUrlForLayer"
			],
			"Condition": {
				"StringEquals": {
					"aws:PrincipalOrgID": "o-qisv7rs9ed"
				}
			}
		}
	]
}
`

// ConfigureRelease runs before release to configure it.
func (h *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {

	team, err := h.getTeam(request.Config["team"])
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	response.AdditionalMetadata["team"] = team

	if err := h.InitReleaseAccountCredentials(request.Env, team); err != nil {
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

	var ecrBuilds []string
	for buildID, reqs := range request.ReleaseRequirements {
		response.Env[buildID] = make(map[string]string)

		for _, need := range reqs.Needs {
			if need == "lambda" {
				response.Env[buildID]["LAMBDA_BUCKET"] = LambdaBucket
				setAWSEnvironmentVariables(response.Env[buildID], &releaseAccountCredentialsValue, Region)
			} else if need == "ecr" {
				ecrBuilds = append(ecrBuilds, buildID)
				setAWSEnvironmentVariables(response.Env[buildID], &releaseAccountCredentialsValue, Region)
			} else {
				fmt.Fprintf(h.ErrorStream, "unable to satisfy %q need for %q build", need, buildID)
				response.Success = false
				return nil
			}
		}
	}
	if len(ecrBuilds) != 0 {
		sort.Strings(ecrBuilds)
		ecrClient := h.ECRClientFactory(session)
		if err := h.setupECR(request.Component, request.Version, team, response, ecrClient, ecrBuilds); err != nil {
			fmt.Fprintln(h.ErrorStream, err)
			response.Success = false
			return nil
		}
	}
	return nil
}

func (h *Handler) setupECR(component, version, team string, response *common.ConfigureReleaseResponse, ecrClient ecriface.ECRAPI, ecrBuilds []string) error {

	repoName := team + "-" + component

	fmt.Fprintf(h.ErrorStream, "- Checking ECR repository...\n")

	repoURI, err := h.getECRRepo(repoName, ecrClient)
	if err != nil {
		return err
	}

	fmt.Fprintf(h.ErrorStream, "- Checking ECR repository policy...\n")

	if err := h.ensureRepoPolicy(repoName, ecrClient); err != nil {
		return err
	}

	fmt.Fprintf(h.ErrorStream, "- Checking ECR lifecycle policy...\n")

	if err := h.ensureECRRepoLifecycle(repoName, ecrBuilds, ecrClient); err != nil {
		return err
	}
	for _, buildID := range ecrBuilds {
		response.Env[buildID]["ECR_REPOSITORY"] = repoURI
		response.Env[buildID]["ECR_TAG"] = buildID + "-" + version
	}

	return nil
}

// ECRLifecyclePolicy represents a lifecycle policy in ECR.
type ECRLifecyclePolicy struct {
	Rules []*ECRLifecyclePolicyRule `json:"rules"`
}

// ECRLifecyclePolicyRule represents a rule in a lifecycle policy in ECR.
type ECRLifecyclePolicyRule struct {
	RulePriority int                              `json:"rulePriority"`
	Selection    *ECRLifecyclePolicyRuleSelection `json:"selection"`
	Action       *ECRLifecyclePolicyRuleAction    `json:"action"`
}

// ECRLifecyclePolicyRuleSelection repesents a selection within an ECR lifecycle policy rule.
type ECRLifecyclePolicyRuleSelection struct {
	TagStatus     string   `json:"tagStatus"`
	TagPrefixList []string `json:"tagPrefixList"`
	CountType     string   `json:"countType"`
	CountNumber   int      `json:"countNumber"`
}

// ECRLifecyclePolicyRuleAction repesents an action within an ECR lifecycle policy rule.
type ECRLifecyclePolicyRuleAction struct {
	Type string `json:"type"`
}

func (h *Handler) ensureECRRepoLifecycle(repoName string, ecrBuilds []string, ecrClient ecriface.ECRAPI) error {
	fmt.Fprintf(h.ErrorStream, "- Fetching lifecycle policy...\n")
	output, err := ecrClient.GetLifecyclePolicy(&ecr.GetLifecyclePolicyInput{
		RepositoryName: aws.String(repoName),
	})
	var existingPolicyText string
	if err != nil {
		if aerr, ok := err.(awserr.Error); !ok || ok && aerr.Code() != ecr.ErrCodeLifecyclePolicyNotFoundException {
			return err
		}
	} else {
		existingPolicyText = *output.LifecyclePolicyText
	}
	policy := &ECRLifecyclePolicy{}
	for i, buildID := range ecrBuilds {
		policy.Rules = append(policy.Rules, &ECRLifecyclePolicyRule{
			RulePriority: i + 1,
			Selection: &ECRLifecyclePolicyRuleSelection{
				TagStatus:     "tagged",
				TagPrefixList: []string{buildID + "-"},
				CountType:     "imageCountMoreThan",
				CountNumber:   50,
			},
			Action: &ECRLifecyclePolicyRuleAction{
				Type: "expire",
			},
		})
	}
	serialisedPolicy, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	if string(serialisedPolicy) == existingPolicyText {
		return nil
	}
	fmt.Fprintf(h.ErrorStream, "- Updating lifecycle policy...\n")
	if _, err := ecrClient.PutLifecyclePolicy(&ecr.PutLifecyclePolicyInput{
		RepositoryName:      aws.String(repoName),
		RegistryId:          aws.String(AccountID),
		LifecyclePolicyText: aws.String(string(serialisedPolicy)),
	}); err != nil {
		return err
	}
	return nil
}

func setAWSEnvironmentVariables(env map[string]string, creds *credentials.Value, region string) {
	env["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
	env["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
	env["AWS_SESSION_TOKEN"] = creds.SessionToken
	// depending on the SDK one of these will be used
	env["AWS_REGION"] = region         // java & go
	env["AWS_DEFAULT_REGION"] = region // python, node, etc.
}

func (h *Handler) getECRRepo(repoName string, ecrClient ecriface.ECRAPI) (string, error) {
	response, err := ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{aws.String(repoName)},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == ecr.ErrCodeRepositoryNotFoundException {
			return h.createECRRepo(repoName, ecrClient)
		}
		return "", err
	}
	if !*response.Repositories[0].ImageScanningConfiguration.ScanOnPush {
		if _, err := ecrClient.PutImageScanningConfiguration(&ecr.PutImageScanningConfigurationInput{
			RepositoryName: aws.String(repoName),
			ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
				ScanOnPush: aws.Bool(true),
			},
		}); err != nil {
			return "", err
		}
	}
	if *response.Repositories[0].ImageTagMutability != "IMMUTABLE" {
		if _, err := ecrClient.PutImageTagMutability(&ecr.PutImageTagMutabilityInput{
			RepositoryName:     aws.String(repoName),
			ImageTagMutability: aws.String("IMMUTABLE"),
		}); err != nil {
			return "", err
		}
	}
	return *response.Repositories[0].RepositoryUri, nil
}

func (h *Handler) ensureRepoPolicy(repoName string, ecrClient ecriface.ECRAPI) error {
	response, err := ecrClient.GetRepositoryPolicy(
		&ecr.GetRepositoryPolicyInput{RepositoryName: aws.String(repoName)})

	if err != nil {
		if aerr, ok := err.(awserr.Error); !ok || ok && aerr.Code() != ecr.ErrCodeRepositoryPolicyNotFoundException {
			return err
		}
	} else {
		if *response.PolicyText == ECR_REPO_POLICY {
			return nil
		}
	}

	if _, err := ecrClient.SetRepositoryPolicy(&ecr.SetRepositoryPolicyInput{
		PolicyText:     aws.String(ECR_REPO_POLICY),
		RepositoryName: aws.String(repoName),
	}); err != nil {
		return err
	}

	return nil
}

func (h *Handler) createECRRepo(name string, ecrClient ecriface.ECRAPI) (string, error) {
	response, err := ecrClient.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(name),
		ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
			ScanOnPush: aws.Bool(true),
		},
		ImageTagMutability: aws.String("IMMUTABLE"),
	})
	if err != nil {
		return "", err
	}
	return *response.Repository.RepositoryUri, nil
}
