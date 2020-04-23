package handler_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

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
	mockS3Client := &MockS3Client{}
	h := handler.New().
		WithErrorStream(&errorBuffer).
		WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
		WithS3ClientFactory(func(client.ConfigProvider) s3iface.S3API {
			return mockS3Client
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
	if len(mockS3Client.putObjectCalls) != 1 {
		t.Fatalf("unexpected number of PutObjectCalls: %d", len(mockS3Client.putObjectCalls))
	}
	call := mockS3Client.putObjectCalls[0]
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
