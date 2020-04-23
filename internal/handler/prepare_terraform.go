package handler

import (
	common "github.com/mergermarket/cdflow2-config-common"
)

// PrepareTerraform runs before terraform to configure.
func (h *Handler) PrepareTerraform(request *common.PrepareTerraformRequest, response *common.PrepareTerraformResponse) error {

	// if err := h.InitReleaseAccountCredentials(request.Env, request.Team); err != nil {
	// 	response.Success = false
	// 	fmt.Fprintln(h.ErrorStream, err)
	// 	return nil
	// }

	// releaseAccountCredentialsValue, err := h.ReleaseAccountCredentials.Get()
	// if err != nil {
	// 	response.Success = false
	// 	fmt.Fprintln(h.ErrorStream, err)
	// 	return nil
	// }

	// // response.TerraformBackendType = "s3"
	// // response.TerraformBackendConfig["bucket"] = ""
	// // response.TerraformBackendConfig["region"] = ""
	// // response.TerraformBackendConfig["key"] = fmt.Sprintf("%s/%s/%s/terraform.tfstate")
	// //response.TerraformBackendConfig["dynamodb_table"] = ""
	// response.TerraformBackendConfig["access_key"] = releaseAccountCredentialsValue.AccessKeyID
	// response.TerraformBackendConfig["secret_key"] = releaseAccountCredentialsValue.SecretAccessKey
	// response.TerraformBackendConfig["token"] = releaseAccountCredentialsValue.SessionToken

	return nil
}
