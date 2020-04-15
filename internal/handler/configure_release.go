package handler

import common "github.com/mergermarket/cdflow2-config-common"

// ConfigureRelease runs before release to configure it.
func (handler *Handler) ConfigureRelease(request *common.ConfigureReleaseRequest, response *common.ConfigureReleaseResponse) error {
	releaseAccount := "acurisrelease"
	response.Env()

	return nil
}
