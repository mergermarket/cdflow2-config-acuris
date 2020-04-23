package handler_test

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

type MockAssumeRoleProvider struct {
	retrieve func() (credentials.Value, error)
}

func (*MockAssumeRoleProvider) IsExpired() bool {
	return false
}

func (m *MockAssumeRoleProvider) Retrieve() (credentials.Value, error) {
	return m.retrieve()
}

func createMockAssumeRoleProvider(accessKeyID, secretAccessKey, sessionToken string) *MockAssumeRoleProvider {
	return &MockAssumeRoleProvider{
		retrieve: func() (credentials.Value, error) {
			return credentials.Value{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				SessionToken:    sessionToken,
			}, nil
		},
	}
}

type MockS3Client struct {
	s3iface.S3API
	putObjectCalls []*s3.PutObjectInput
}

func (m *MockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	m.putObjectCalls = append(m.putObjectCalls, input)
	return &s3.PutObjectOutput{}, nil
}
