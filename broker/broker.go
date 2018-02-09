package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	"github.com/pivotal-cf/brokerapi"
)

const (
	PlanNameSimple  = "simple"
	PlanNameComplex = "complex"
	BrokerID        = "sapi-credhub-broker"
)

type InstanceCredentials struct {
	Host     string
	Port     int
	Password string
}

type InstanceCreator interface {
	Create() error
	Destroy() error
	InstanceExists() (bool, error)
}

type InstanceBinder interface {
	Bind() (InstanceCredentials, error)
	Unbind() error
	InstanceExists() (bool, error)
}

type CredhubServiceBroker struct {
	InstanceCreators map[string]InstanceCreator
	InstanceBinders  map[string]InstanceBinder
	CredHubClient    *credhub.CredHub
	Logger           lager.Logger
}

func (credhubServiceBroker *CredhubServiceBroker) Services(context context.Context) []brokerapi.Service {
	fmt.Println("this is the catalog endpoint")
	planList := []brokerapi.ServicePlan{}
	for _, plan := range credhubServiceBroker.plans() {
		planList = append(planList, *plan)
	}

	return []brokerapi.Service{
		brokerapi.Service{
			ID:          BrokerID,
			Name:        "awesome-credhub-broker",
			Description: "stores binding config params in credhub",
			Bindable:    true,
			Plans:       planList,
			Metadata: &brokerapi.ServiceMetadata{
				DisplayName:         "credhub-broker",
				LongDescription:     "stores binding config params in credhub",
				DocumentationUrl:    "http://example.com",
				SupportUrl:          "",
				ImageUrl:            "",
				ProviderDisplayName: "",
			},
			Tags: []string{
				"sapi",
				"credhub",
			},
		},
	}
}

func (credhubServiceBroker *CredhubServiceBroker) Provision(context context.Context, instanceID string, serviceDetails brokerapi.ProvisionDetails, asyncAllowed bool) (spec brokerapi.ProvisionedServiceSpec, err error) {
	err = credhubServiceBroker.setJSON(serviceDetails.RawParameters, instanceID, serviceDetails.ServiceID)

	if err != nil {
		return spec, err
	}

	credhubServiceBroker.Logger.Info("successfully stored user-provided credentials for key " + instanceID)
	return spec, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	var actor string
	if details.AppGUID != "" {
		actor = fmt.Sprintf("mtls-app:%s", details.AppGUID)
	}

	if actor == "" {
		return brokerapi.Binding{}, errors.New("No app-guid or credential client ID were provided in the binding request, you must configure one of these")
	}

	additionalPermissions := []permissions.Permission{
		{
			Actor:      actor,
			Operations: []string{"read"},
		},
	}

	key := constructKey(details.ServiceID, instanceID)
	credhubServiceBroker.CredHubClient.AddPermissions(key, additionalPermissions)

	bindResponse := brokerapi.Binding{}
	bindResponse.Credentials = map[string]string{"credhub-ref": key}
	return bindResponse, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	return nil
}

// LastOperation ...
func (credhubServiceBroker *CredhubServiceBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Update(context context.Context, instanceID string, serviceDetails brokerapi.UpdateDetails, asyncAllowed bool) (spec brokerapi.UpdateServiceSpec, err error) {
	err = credhubServiceBroker.setJSON(serviceDetails.RawParameters, instanceID, serviceDetails.ServiceID)

	if err != nil {
		return spec, err
	}

	credhubServiceBroker.Logger.Info("successfully updated user-provided credentials for instance " + instanceID)
	return spec, nil
}

func (credhubServiceBroker *CredhubServiceBroker) plans() map[string]*brokerapi.ServicePlan {
	plans := map[string]*brokerapi.ServicePlan{}

	plans["simple"] = &brokerapi.ServicePlan{
		ID:          "simple-id",
		Name:        "simple-plan",
		Description: "This plan provides a single Redis process on a shared VM, which is suitable for development and testing workloads",
		Metadata: &brokerapi.ServicePlanMetadata{
			Bullets: []string{
				"Each instance shares the same VM",
				"Single dedicated Redis process",
				"Suitable for development & testing workloads",
			},
			DisplayName: "simple",
		},
	}

	return plans
}

func constructKey(serviceID, instanceID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", BrokerID, serviceID, instanceID)
}

func (credhubServiceBroker *CredhubServiceBroker) setJSON(rawParameters json.RawMessage, instanceID string, serviceID string) (err error) {
	var credentials map[string]interface{}
	err = json.Unmarshal(rawParameters, &credentials)
	if err != nil {
		return brokerapi.ErrRawParamsInvalid
	}

	key := constructKey(serviceID, instanceID)
	_, err = credhubServiceBroker.CredHubClient.SetJSON(key, values.JSON(credentials), credhub.Mode("overwrite"))

	if err != nil {
		credhubServiceBroker.Logger.Error("store user-provided credentials to credhub ", err, map[string]interface{}{"key": key})
		return brokerapi.NewFailureResponse(err, http.StatusInternalServerError, "unable to store the user-provided credentials")
	}

	return nil
}
