package main

import (
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/ablease/credhub-broker/broker"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/util"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	brokerLogger := lager.NewLogger("credhub-broker")
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))
	brokerLogger.Info("starting up the credhub broker...")

	credHubClient := authenticate()
	serviceBroker := &broker.CredhubServiceBroker{CredHubClient: credHubClient, Logger: brokerLogger}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: "admin",
		Password: "admin",
	}

	brokerAPI := brokerapi.New(serviceBroker, brokerLogger, brokerCredentials)

	http.Handle("/", brokerAPI)

	brokerLogger.Fatal("http-listen", http.ListenAndServe("localhost"+":"+"3000", nil))
}

func authenticate() *credhub.CredHub {

	skipTLSValidation := false
	if skipTLS := os.Getenv("SKIP_TLS_VALIDATION"); skipTLS == "true" {
		skipTLSValidation = true
	}

	ch, err := credhub.New(
		util.AddDefaultSchemeIfNecessary(os.Getenv("CREDHUB_SERVER")),
		credhub.SkipTLSValidation(skipTLSValidation),
		credhub.Auth(auth.UaaClientCredentials(os.Getenv("CREDHUB_CLIENT"), os.Getenv("CREDHUB_SECRET"))),
	)
	if err != nil {
		panic("credhub client configured incorrectly: " + err.Error())
	}

	return ch
}
