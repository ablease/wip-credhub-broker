package main

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/ablease/credhub-broker/broker"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	brokerLogger := lager.NewLogger("credhub-broker")
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))
	brokerLogger.Info("starting up the credhub broker...")

	serviceBroker := &broker.CredhubServiceBroker{}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: "admin",
		Password: "admin",
	}

	brokerAPI := brokerapi.New(serviceBroker, brokerLogger, brokerCredentials)

	http.Handle("/", brokerAPI)

	fmt.Println("this is the broker starting up")
	brokerLogger.Fatal("http-listen", http.ListenAndServe("localhost"+":"+"3000", nil))
}
