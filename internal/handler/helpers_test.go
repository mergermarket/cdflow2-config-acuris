package handler_test

import (
	"errors"
	"io"
	"path"

	"github.com/aws/aws-sdk-go/aws"
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
	files                  map[string][]byte
	putObjectCalls         []*s3.PutObjectInput
	getObjectBody          io.ReadCloser
	getObjectContentLength int64
	headObjectMetadata     map[string]*string
}

func (m *MockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	m.putObjectCalls = append(m.putObjectCalls, input)
	return &s3.PutObjectOutput{}, nil
}

func (m *MockS3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{
		Body:          m.getObjectBody,
		ContentLength: aws.Int64(m.getObjectContentLength),
	}, nil
}

func (m *MockS3Client) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	// m.mu.Lock()
	// defer m.mu.Unlock()

	key := path.Join(*input.Bucket, *input.Key)
	if _, ok := m.files[key]; !ok {
		return nil, errors.New("- Key does not exist")
	}

	// return &s3.HeadObjectOutput{
	// 	Body: ioutil.NopCloser(bytes.NewReader(m.files[key])),
	// }, nil
	return &s3.HeadObjectOutput{
		Metadata: m.headObjectMetadata,
	}, nil
}
