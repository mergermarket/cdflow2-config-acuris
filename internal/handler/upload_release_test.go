package handler_test

import (
	"bytes"
	"io"
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

type MockReleaseSaver struct {
	called         bool
	component      string
	version        string
	terraformImage string
	releaseDir     string
	reader         io.ReadCloser
}

func (m *MockReleaseSaver) Save(
	component, version, terraformImage, releaseDir string,
	pluginSaver func(path, checksum string, reader io.ReadCloser) error,
) (io.ReadCloser, error) {
	if m.called {
		log.Fatal("saver called twice")
	}
	m.called = true
	m.component = component
	m.version = version
	m.terraformImage = terraformImage
	m.releaseDir = releaseDir
	return m.reader, nil
}

func TestUploadRelease(t *testing.T) {
	// Given
	request := common.CreateUploadReleaseRequest()
	response := common.CreateUploadReleaseResponse()
	configureReleaseRequest := common.CreateConfigureReleaseRequest()
	team := "test-team"
	component := "test-component"
	version := "test-version"
	terraformImage := "test-terraform-image"
	configureReleaseRequest.Config["team"] = team
	configureReleaseRequest.Component = component
	configureReleaseRequest.Version = version
	request.TerraformImage = terraformImage

	mockAssumeRoleProviderFactory := func(session client.ConfigProvider, roleARN, roleSessionName string) credentials.Provider {
		return createMockAssumeRoleProvider("foo", "bar", "baz")
	}

	file, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())
	saver := MockReleaseSaver{reader: file}

	var errorBuffer bytes.Buffer
	mockS3Uploader := &MockS3Uploader{}
	h := handler.New().
		WithErrorStream(&errorBuffer).
		WithAssumeRoleProviderFactory(mockAssumeRoleProviderFactory).
		WithS3UploaderFactory(func(client.ConfigProvider) s3manageriface.UploaderAPI {
			return mockS3Uploader
		}).
		WithReleaseSaver(&saver)

	// normally this would have happened as part of the configure release
	h.InitReleaseAccountCredentials(map[string]string{}, "test-team")

	releaseDir, err := ioutil.TempDir("", "cdflow2-config-acuris-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(releaseDir)

	// When
	if err := h.UploadRelease(request, response, configureReleaseRequest, releaseDir); err != nil {
		t.Fatal("error in upload release:", err)
	}

	// Then
	if saver.releaseDir != releaseDir {
		t.Fatalf("expected %s, got %s", releaseDir, saver.releaseDir)
	}
	if saver.terraformImage != terraformImage {
		t.Fatalf("expected %s, got %s", terraformImage, saver.terraformImage)
	}
	if saver.component != component {
		t.Fatalf("expected %s, got %s", component, saver.component)
	}
	if saver.version != version {
		t.Fatalf("expected %s, got %s", version, saver.version)
	}
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
