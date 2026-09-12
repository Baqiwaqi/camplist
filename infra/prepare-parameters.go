//go:build ignore

// Initial bootstrap helper. It reads .env without sourcing it as shell code.
// Cookie keys are freshly generated: don't use this to redeploy an existing app.
// Routine deployments update the image and preserve the secrets already in ACA.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: go run infra/prepare-parameters.go OUTPUT REGISTRY IMAGE")
	}
	values, err := godotenv.Read(".env")
	if err != nil {
		panic(err)
	}
	params := map[string]any{"registryName": map[string]string{"value": os.Args[2]}, "image": map[string]string{"value": os.Args[3]}}
	for param, key := range map[string]string{"dbUrl": "DB_URL", "dbKey": "DB_KEY", "googleClientId": "GOOGLE_CLIENT_ID", "googleClientSecret": "GOOGLE_CLIENT_SECRET"} {
		if values[key] == "" {
			panic(key + " is required in .env")
		}
		params[param] = map[string]string{"value": values[key]}
	}
	for _, key := range []string{"sessionKey", "csrfKey"} {
		value := make([]byte, 32)
		if _, err := rand.Read(value); err != nil {
			panic(err)
		}
		params[key] = map[string]string{"value": base64.RawURLEncoding.EncodeToString(value)}
	}
	file, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = json.NewEncoder(file).Encode(map[string]any{"$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#", "contentVersion": "1.0.0.0", "parameters": params})
	if err != nil {
		panic(err)
	}
	fmt.Println("Created protected bootstrap parameters; remove this file after deployment.")
}
