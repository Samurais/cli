package api

import (
	"cf/configuration"
	"cf/models"
	"cf/net"
	"fmt"
	"net/url"
	"strings"
)

type PaginatedQuotaResources struct {
	Resources []QuotaResource
}

type QuotaResource struct {
	Resource
	Entity QuotaEntity
}

func (resource QuotaResource) ToFields() (quota models.QuotaFields) {
	quota.Guid = resource.Metadata.Guid
	quota.Name = resource.Entity.Name
	quota.MemoryLimit = resource.Entity.MemoryLimit
	return
}

type QuotaEntity struct {
	Name        string
	MemoryLimit uint64 `json:"memory_limit"`
}

type QuotaRepository interface {
	FindAll() (quotas []models.QuotaFields, apiResponse net.ApiResponse)
	FindByName(name string) (quota models.QuotaFields, apiResponse net.ApiResponse)
	Update(orgGuid, quotaGuid string) (apiResponse net.ApiResponse)
}

type CloudControllerQuotaRepository struct {
	config  configuration.Reader
	gateway net.Gateway
}

func NewCloudControllerQuotaRepository(config configuration.Reader, gateway net.Gateway) (repo CloudControllerQuotaRepository) {
	repo.config = config
	repo.gateway = gateway
	return
}

func (repo CloudControllerQuotaRepository) findAllWithPath(path string) (quotas []models.QuotaFields, apiResponse net.ApiResponse) {
	resources := new(PaginatedQuotaResources)

	apiResponse = repo.gateway.GetResource(path, repo.config.AccessToken(), resources)
	if apiResponse.IsNotSuccessful() {
		return
	}

	for _, r := range resources.Resources {
		quotas = append(quotas, r.ToFields())
	}

	return
}

func (repo CloudControllerQuotaRepository) FindAll() (quotas []models.QuotaFields, apiResponse net.ApiResponse) {
	path := fmt.Sprintf("%s/v2/quota_definitions", repo.config.ApiEndpoint())
	return repo.findAllWithPath(path)
}

func (repo CloudControllerQuotaRepository) FindByName(name string) (quota models.QuotaFields, apiResponse net.ApiResponse) {
	path := fmt.Sprintf("%s/v2/quota_definitions?q=%s", repo.config.ApiEndpoint(), url.QueryEscape("name:"+name))
	quotas, apiResponse := repo.findAllWithPath(path)
	if apiResponse.IsNotSuccessful() {
		return
	}

	if len(quotas) == 0 {
		apiResponse = net.NewNotFoundApiResponse("Quota '%s' not found", name)
		return
	}

	quota = quotas[0]
	return
}

func (repo CloudControllerQuotaRepository) Update(orgGuid, quotaGuid string) (apiResponse net.ApiResponse) {
	path := fmt.Sprintf("%s/v2/organizations/%s", repo.config.ApiEndpoint(), orgGuid)
	data := fmt.Sprintf(`{"quota_definition_guid":"%s"}`, quotaGuid)
	return repo.gateway.UpdateResource(path, repo.config.AccessToken(), strings.NewReader(data))
}
