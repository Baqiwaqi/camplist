package db

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

type Client struct {
	cosmos    *azcosmos.Client
	container *azcosmos.ContainerClient
}

func NewClient(endpoint string, key string, dbName string, containerName string) (*Client, error) {
	credential, err := azcosmos.NewKeyCredential(key)
	if err != nil {
		return nil, err
	}
	clientOptions := azcosmos.ClientOptions{
		EnableContentResponseOnWrite: true,
	}

	client, err := azcosmos.NewClientWithKey(endpoint, credential, &clientOptions)
	if err != nil {
		return nil, err
	}

	// create database
	database, err := client.NewDatabase(dbName)
	if err != nil {
		return nil, err
	}

	// create the container
	container, err := client.NewContainer(database.ID(), containerName)
	if err != nil {
		return nil, err
	}

	// Enable per-item TTL without applying a default expiry to packing data.
	// Share-link items carry their own seven-day ttl value.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	response, err := container.Read(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("read Cosmos container TTL configuration: %w", err)
	}
	if response.ContainerProperties.DefaultTimeToLive == nil {
		itemTTLOnly := int32(-1)
		response.ContainerProperties.DefaultTimeToLive = &itemTTLOnly
		if _, err = container.Replace(ctx, *response.ContainerProperties, nil); err != nil {
			return nil, fmt.Errorf("enable per-item Cosmos TTL: %w", err)
		}
	}

	return &Client{
		client,
		container,
	}, nil
}
func (c *Client) Container() *azcosmos.ContainerClient {
	return c.container
}
