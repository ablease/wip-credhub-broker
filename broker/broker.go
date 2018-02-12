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
	PlanNameDefault = "default"
	BrokerID        = "secure-credentials-broker"
	ServiceID       = "secure-credentials"
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
		brokerapi.Service{Name: ServiceID,
			Description:   "Stores configuration parameters securely in CredHub",
			Bindable:      true,
			PlanUpdatable: false,
			Plans:         planList,
			Metadata: &brokerapi.ServiceMetadata{
				DisplayName:         "credhub-broker",
				LongDescription:     "Stores configuration parameters securely in CredHub",
				DocumentationUrl:    "",
				SupportUrl:          "",
				ImageUrl:            "",
				ProviderDisplayName: "",
			},
			Tags: []string{
				"credhub",
			},
		},
	}
}

func (credhubServiceBroker *CredhubServiceBroker) Provision(context context.Context, instanceID string, serviceDetails brokerapi.ProvisionDetails, asyncAllowed bool) (spec brokerapi.ProvisionedServiceSpec, err error) {
	var credentials map[string]interface{}

	err = json.Unmarshal(serviceDetails.RawParameters, &credentials)
	if err != nil {
		return spec, brokerapi.NewFailureResponse(
			errors.New("Configuration parameters containing the credentials you wish to store in CredHub are needed to use this service"),
			http.StatusUnprocessableEntity, "missing-parameters")
	}

	err = credhubServiceBroker.setJSON(credentials, instanceID, serviceDetails.ServiceID)

	if err != nil {
		return spec, err
	}

	credhubServiceBroker.Logger.Info("successfully stored user-provided credentials for key " + instanceID)
	return spec, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	serviceInstanceKey := constructKey(details.ServiceID, instanceID)

	err := credhubServiceBroker.CredHubClient.Delete(serviceInstanceKey)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	serviceKey := constructKey(details.ServiceID, instanceID)

	credentialsJSON, err := credhubServiceBroker.CredHubClient.GetLatestJSON(serviceKey)
	if err != nil {
		return brokerapi.Binding{}, brokerapi.NewFailureResponse(
			errors.New("Unable to retrieve service instance credentials from CredHub"),
			http.StatusInternalServerError, "missing-service-instance-entry")
	}

	credhubServiceBroker.setJSON(credentialsJSON.Value, bindingID, details.ServiceID)
	if err != nil {
		return brokerapi.Binding{}, err
	}

	if details.AppGUID == "" {
		return brokerapi.Binding{}, errors.New("No app-guid or credential client ID were provided in the binding request, you must configure one of these")
	}
	actor := fmt.Sprintf("mtls-app:%s", details.AppGUID)
	additionalPermissions := []permissions.Permission{
		{
			Actor:      actor,
			Operations: []string{"read"},
		},
	}

	bindingKey := constructKey(details.ServiceID, bindingID)
	_, err = credhubServiceBroker.CredHubClient.AddPermissions(bindingKey, additionalPermissions)
	if err != nil {
		return brokerapi.Binding{}, err
	}

	return brokerapi.Binding{Credentials: map[string]string{"credhub-ref": bindingKey}}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	bindingKey := constructKey(details.ServiceID, bindingID)

	err := credhubServiceBroker.CredHubClient.Delete(bindingKey)
	if err != nil {
		return err
	}

	return nil
}

// LastOperation ...
func (credhubServiceBroker *CredhubServiceBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Update(context context.Context, instanceID string, serviceDetails brokerapi.UpdateDetails, asyncAllowed bool) (spec brokerapi.UpdateServiceSpec, err error) {
	return spec, brokerapi.ErrPlanChangeNotSupported
}

func (credhubServiceBroker *CredhubServiceBroker) plans() map[string]*brokerapi.ServicePlan {
	plans := map[string]*brokerapi.ServicePlan{}

	plans[PlanNameDefault] = &brokerapi.ServicePlan{
		ID:          PlanNameDefault,
		Name:        PlanNameDefault,
		Description: "Stores configuration parameters securely in CredHub",
		Metadata: &brokerapi.ServicePlanMetadata{
			Bullets: []string{
				"Stores configuration parameters securely in CredHub",
			},
			DisplayName: PlanNameDefault,
		},
	}

	return plans
}

func constructKey(serviceID, instanceID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", BrokerID, serviceID, instanceID)
}

func (credhubServiceBroker *CredhubServiceBroker) setJSON(credentials map[string]interface{}, instanceID string, serviceID string) (err error) {

	key := constructKey(serviceID, instanceID)
	_, err = credhubServiceBroker.CredHubClient.SetJSON(key, values.JSON(credentials), credhub.Mode("no-overwrite"))

	if err != nil {
		credhubServiceBroker.Logger.Error("unable to store credentials to credhub ", err, map[string]interface{}{"key": key})
		return brokerapi.NewFailureResponse(err, http.StatusInternalServerError, "Unable to store the provided credentials.")
	}

	return nil
}
