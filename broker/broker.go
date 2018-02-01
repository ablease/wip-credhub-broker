package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/pivotal-cf/brokerapi"
)

const (
	PlanNameSimple  = "simple"
	PlanNameComplex = "complex"
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
}

func (credhubServiceBroker *CredhubServiceBroker) Services(context context.Context) []brokerapi.Service {
	fmt.Println("this is the catalog endpoint")
	planList := []brokerapi.ServicePlan{}
	for _, plan := range credhubServiceBroker.plans() {
		planList = append(planList, *plan)
	}

	return []brokerapi.Service{
		brokerapi.Service{
			ID:          "sapi-credhub-broker",
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

//Provision ...
func (credhubServiceBroker *CredhubServiceBroker) Provision(context context.Context, instanceID string, serviceDetails brokerapi.ProvisionDetails, asyncAllowed bool) (spec brokerapi.ProvisionedServiceSpec, err error) {

	// initialize a credhub-cli client
	// set some credentials into credhub
	ch, err := credhub.New("<CREDHUB_URL>",
		credhub.SkipTLSValidation(true),
		credhub.Auth(auth.UaaClientCredentials("CREDHUB_CLIENT", "CREDHUB_CLIENT_SECRET")))
	if err != nil {
		panic("credhub client configured incorrectly: " + err.Error())
	}

	authUrl, err := ch.AuthURL()
	if err != nil {
		panic("couldn't fetch authurl")
	}

	fmt.Println("CredHub server: ", ch.ApiURL)
	fmt.Println("Auth server: ", authUrl)

	// Set configuration parameters into credhub
	var m map[string]interface{}
	err = json.Unmarshal(serviceDetails.RawParameters, &m)
	key := serviceDetails.ServiceID + "/" + instanceID
	resp, err := ch.SetJSON(key, values.JSON(m), credhub.Mode("no-overwrite"))
	if err != nil {
		fmt.Println("Error setting JSON in credhub")
	}

	fmt.Println("Response from SetJSON: ", resp)

	return spec, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	fmt.Println("---------- about to do the Credential mapping")
	key := details.ServiceID + "/" + instanceID
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

	plans["complex"] = &brokerapi.ServicePlan{
		ID:          "complex-id",
		Name:        "complex-name",
		Description: "This plan provides a single Redis process on a dedicated VM, which is suitable for production workloads",
		Metadata: &brokerapi.ServicePlanMetadata{
			Bullets: []string{
				"Dedicated VM per instance",
				"Single dedicated Redis process",
				"Suitable for production workloads",
			},
			DisplayName: "Dedicated-VM",
		},
	}

	return plans
}

func (credhubServiceBroker *CredhubServiceBroker) instanceExists() bool {
	return false
}

// LastOperation ...
func (credhubServiceBroker *CredhubServiceBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func (credhubServiceBroker *CredhubServiceBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, nil
}
