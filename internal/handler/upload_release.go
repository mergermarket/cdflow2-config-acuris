package handler

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	common "github.com/mergermarket/cdflow2-config-common"
)

// UploadRelease runs after release to upload the release., releaseReader io.ReadSeeker
func (h Handler) UploadRelease(request *common.UploadReleaseRequest, response *common.UploadReleaseResponse, configureReleaseRequest *common.ConfigureReleaseRequest, releaseReader io.ReadSeeker) error {

	session, err := h.createReleaseAccountSession()
	if err != nil {
		return fmt.Errorf("unable to create AWS session in release account: %v", err)
	}

	s3Client := h.S3ClientFactory(session)
	key := releaseS3Key(configureReleaseRequest.Team, configureReleaseRequest.Component, configureReleaseRequest.Version)
	if _, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(ReleaseBucket),
		Key:    aws.String(key),
		Body:   releaseReader,
	}); err != nil {
		fmt.Fprintln(h.ErrorStream, "unable to upload release to S3:", err)
		response.Success = false
		return nil
	}

	fmt.Fprintf(h.ErrorStream, "Release uploaded to s3://%s/%s.\n", ReleaseBucket, key)

	return nil
}
