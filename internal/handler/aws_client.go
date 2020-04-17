package handler

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
)

type AWSClientSDK struct {
	env      map[string]string
	session  client.ConfigProvider
	ecrRepos map[string]string
}

func getAWSClientSDKFactory() AWSClientFactory {
	return func(env map[string]string) AWSClient {
		return &AWSClientSDK{env: env}
	}
}

func (a *AWSClientSDK) STSAssumeRole(team string) (*sts.Credentials, error) {
	session, err := a.getSession()
	if err != nil {
		return nil, err
	}
	stsClient := sts.New(session)
	response, err := stsClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s-deploy", AccountID, team)),
		RoleSessionName: aws.String("role-session-name"), // @todo let's get a better session name
	})
	if err != nil {
		return nil, err
	}
	return response.Credentials, err
}

func (a *AWSClientSDK) GetECRRepoURI(componentName string) (string, error) {
	if a.ecrRepos[componentName] != "" {
		return a.ecrRepos[componentName], nil
	}

	session, err := a.getSession()
	if err != nil {
		return "", err
	}
	ecrClient := ecr.New(session)
	response, err := ecrClient.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{aws.String(componentName)},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == ecr.ErrCodeRepositoryNotFoundException {
			return "", fmt.Errorf("ECR repository for %s does not exist", componentName)
		}
		return "", err
	}
	uri := *response.Repositories[0].RepositoryUri
	a.ecrRepos[componentName] = uri
	return uri, nil
}

func (a *AWSClientSDK) getSession() (client.ConfigProvider, error) {
	if a.session != nil {
		return a.session, nil
	}
	id := a.env["AWS_ACCESS_KEY_ID"]
	secret := a.env["AWS_SECRET_ACCESS_KEY"]
	if id == "" || secret == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not found in env")
	}
	creds := credentials.NewStaticCredentials(id, secret, a.env["AWS_SESSION_TOKEN"])
	session, err := session.NewSession(aws.NewConfig().WithCredentials(creds).WithRegion("eu-west-1"))
	if err != nil {
		return nil, fmt.Errorf("unable to create a new AWS session: %v", err)
	}
	return session, err
}
