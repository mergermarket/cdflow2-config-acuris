package handler_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/mergermarket/cdflow2-config-acuris/internal/handler"
	common "github.com/mergermarket/cdflow2-config-common"
)

type mockedSTS struct {
	stsiface.STSAPI
}

func TestConfigureRelease(t *testing.T) {
	// Given

	request := common.CreateConfigureReleaseRequest()
	response := common.CreateConfigureReleaseResponse()

	handler := handler.New(&handler.Opts{
		STSClient: mockedSTS{},
	})

	// When
	handler.ConfigureRelease(request, response)

	// Then
	if !response.Success {
		t.Fatal("unexpected failure")
	}
}
