package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
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
	var credentials map[string]interface{}
	err = json.Unmarshal(serviceDetails.RawParameters, &credentials)
	if err != nil {
		return spec, brokerapi.ErrRawParamsInvalid
	}

	key := constructKey(serviceDetails.ServiceID, instanceID)
	_, err = credhubServiceBroker.CredHubClient.SetJSON(key, values.JSON(credentials), credhub.Mode("no-overwrite"))

	if err != nil {
		credhubServiceBroker.Logger.Error("store user-provided credentials to credhub ", err, map[string]interface{}{"key": key})
		return spec, brokerapi.NewFailureResponse(err, http.StatusInternalServerError, "unable to store the user-provided credentials")
	}

	credhubServiceBroker.Logger.Info("Successfully stored user-provided credentials for key " + key)
	return spec, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	key := constructKey(details.ServiceID, instanceID)
	bindResponse := brokerapi.Binding{}
	bindResponse.Credentials = map[string]string{"credhub-ref": key}
	return bindResponse, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	return nil
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

// LastOperation ...
func (credhubServiceBroker *CredhubServiceBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, nil
}

func constructKey(serviceID, instanceID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", BrokerID, serviceID, instanceID)
}
