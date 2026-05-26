package db

import "github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

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

	return &Client{
		client,
		container,
	}, nil
}
func (c *Client) Container() *azcosmos.ContainerClient {
	return c.container
}
