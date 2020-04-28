package handler_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type MockS3Uploader struct {
	s3manageriface.UploaderAPI
	calls []*s3manager.UploadInput
}

func (m *MockS3Uploader) Upload(input *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	m.calls = append(m.calls, input)
	return &s3manager.UploadOutput{}, nil
}

func TestUploadRelease(t *testing.T) {
	// Given
	request := common.CreateUploadReleaseRequest()
	response := common.CreateUploadReleaseResponse()
	configureReleaseRequest := common.CreateConfigureReleaseRequest()
	configureReleaseRequest.Team = "test-team"
	configureReleaseRequest.Component = "test-component"
	configureReleaseRequest.Version = "test-version"

	mockAssumeRoleProviderFactory := func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
		return createMockAssumeRoleProvider("foo", "bar", "baz")
	}

	var errorBuffer bytes.Buffer
	mockS3Uploader := &MockS3Uploader{}
	h := handler.New().
		WithErrorStream(&errorBuffer).
		WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
		WithS3UploaderFactory(func(client.ConfigProvider) s3manageriface.UploaderAPI {
			return mockS3Uploader
		})

	// normally this would have happened as part of the configure release
	h.InitReleaseAccountCredentials(map[string]string{}, "test-team")

	file, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	// When
	h.UploadRelease(request, response, configureReleaseRequest, file)

	// Then
	if len(mockS3Uploader.calls) != 1 {
		t.Fatalf("unexpected number of calls: %d", len(mockS3Uploader.calls))
	}
	call := mockS3Uploader.calls[0]
	if *call.Bucket != handler.ReleaseBucket {
		t.Fatalf("got %q, expected %q", *call.Bucket, handler.ReleaseBucket)
	}
	expectedKey := "test-team/test-component/test-component-test-version.zip"
	if *call.Key != expectedKey {
		t.Fatalf("got %q, expected %q", *call.Key, expectedKey)
	}
	if call.Body != file {
		t.Fatalf("expected file to be passed to PutObject")
	}
}
