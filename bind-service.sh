#/usr/bin/env bash

set -x

curl http://admin:admin@localhost:3000/v2/service_instances/abc123/service_bindings/bindingid -d '{
  "service_id": "simple-id",
  "plan_id": "simple-plan",
  "context": {
    "platform": "cloudfoundry",
    "some_field": "some-contextual-data"
  },
  "app_guid": "app-guid-here",
  "organization_guid": "org-guid-here",
  "space_guid": "space-guid-here",
  "bind_resource": {
    "app_guid": "app-guid-here"
  }
}' -X PUT -H "X-Broker-API-Version: 2.13" -H "Content-Type: application/json"
