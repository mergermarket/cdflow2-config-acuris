package handler

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	common "github.com/mergermarket/cdflow2-config-common"
)

// UploadRelease runs after release to upload the release., releaseReader io.ReadSeeker
func (h *Handler) UploadRelease(request *common.UploadReleaseRequest, response *common.UploadReleaseResponse, configureReleaseRequest *common.ConfigureReleaseRequest, releaseDir string) error {
	team, err := h.getTeam(configureReleaseRequest.Config["team"])
	if err != nil {
		response.Success = false
		fmt.Fprintln(h.ErrorStream, err)
		return nil
	}

	session, err := h.createReleaseAccountSession()
	if err != nil {
		return fmt.Errorf("Unable to create AWS session in release account: %v", err)
	}

	fmt.Fprintf(h.ErrorStream, "Uploading release...\n")

	s3Uploader := h.S3UploaderFactory(session)
	s3Client := h.S3ClientFactory(session)
	key := releaseS3Key(team, configureReleaseRequest.Component, configureReleaseRequest.Version)
	releaseReader, err := h.ReleaseSaver.Save(
		configureReleaseRequest.Component,
		configureReleaseRequest.Version,
		request.TerraformImage,
		releaseDir,
		h.getSubResourceUploader(team, s3Uploader, s3Client),
	)
	if err != nil {
		return err
	}
	defer releaseReader.Close()
	if _, err := s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(ReleaseBucket),
		Key:    aws.String(key),
		Body:   releaseReader,
	}); err != nil {
		fmt.Fprintln(h.ErrorStream, "Unable to upload release to S3:", err)
		response.Success = false
		return nil
	}

	fmt.Fprintf(h.ErrorStream, "Release uploaded to s3://%s/%s.\n", ReleaseBucket, key)

	return nil
}

func (h *Handler) getSubResourceUploader(team string, s3Uploader s3manageriface.UploaderAPI, s3Client s3iface.S3API) func(string, string, io.ReadCloser) error {
	return func(path, checksum string, reader io.ReadCloser) error {
		bucket := aws.String(ReleaseBucket)
		key := aws.String(savedPluginKey(team, path, checksum))
		_, err := s3Client.HeadObject(&s3.HeadObjectInput{
			Bucket: bucket,
			Key:    key,
		})
		if err == nil {
			return nil
		}
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() != s3.ErrCodeNoSuchKey && aerr.Code() != "NotFound" {
				return aerr
			}
		} else {
			return err
		}
		fmt.Fprintf(h.ErrorStream, "Saving provider plugin %s...\n", path)
		if _, err := s3Uploader.Upload(&s3manager.UploadInput{
			Bucket: bucket,
			Key:    key,
			Body:   reader,
		}); err != nil {
			return err
		}
		fmt.Fprintf(h.ErrorStream, "Provider plugin %s saved.\n", path)
		return nil
	}
}
