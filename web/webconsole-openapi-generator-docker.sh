#! /bin/bash

# use Docker to run OpenAPI Generator
docker run --rm -v $PWD:/local openapitools/openapi-generator-cli generate -i /local/webconsole-openapi.yaml -g typescript-axios -o /local/frontend/src/webconsoleApi
