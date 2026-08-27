// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"io"

	"github.com/orca-ae/orca-sdk-go/option"
)

// CloudService is the StreamNative Cloud extension surface, served under
// /apis/cloud.sn.io/v1 by the cloud extension service rather than by the core
// engine.
//
// Every call through this tree is gated on the deployment advertising the
// cloud.sn.io group, and returns [ExtensionNotAvailableError] when it does not.
// That check is what turns "this deployment has no connections at all" into a
// distinct answer, rather than a 404 from a path the caller has no reason to
// doubt. Discovery is resolved once per deployment and cached.
//
// The pre-existing NewCatalogClient-style constructors reach the same
// operations without the gate, and remain for callers that already gate for
// themselves.
type CloudService struct {
	client *Client

	// Agents exposes agent-provider discovery.
	Agents CloudAgentService

	// APIResources lists the resources the cloud.sn.io/v1 group advertises.
	APIResources CloudAPIResourceService

	// Catalog exposes the built-in connector catalog.
	Catalog CloudCatalogService

	// Connections manages external connection definitions.
	Connections CloudConnectionService

	// Connectors manages sink, source, and Kafka Connect connectors.
	Connectors CloudConnectorService

	// Functions manages functions and their instances.
	Functions CloudFunctionService

	// Health reports cloud extension service health.
	Health CloudHealthService

	// Packages manages workspace packages.
	Packages CloudPackageService
}

func newCloudService(client *Client) CloudService {
	return CloudService{
		client:       client,
		Agents:       CloudAgentService{Providers: CloudAgentProviderService{client: client, inner: NewProvidersClient(client)}},
		APIResources: CloudAPIResourceService{client: client},
		Catalog: CloudCatalogService{
			Kafka:   CloudCatalogKafkaService{client: client, inner: NewCatalogClient(client)},
			Sinks:   CloudCatalogSinkService{client: client, inner: NewCatalogClient(client)},
			Sources: CloudCatalogSourceService{client: client, inner: NewCatalogClient(client)},
		},
		Connections: CloudConnectionService{client: client, inner: NewConnectionsClient(client)},
		Connectors: CloudConnectorService{
			Sinks:   CloudConnectorSinkService{client: client, inner: NewSinksClient(client)},
			Sources: CloudConnectorSourceService{client: client, inner: NewSourcesClient(client)},
			Kafka:   CloudConnectorKafkaService{client: client, inner: NewKafkaConnectClient(client)},
		},
		Functions: CloudFunctionService{client: client, inner: NewFunctionsClient(client)},
		Health:    CloudHealthService{client: client, inner: NewHealthClient(client)},
		Packages:  CloudPackageService{client: client, inner: NewPackagesClient(client)},
	}
}

// EnsureAvailable reports whether this deployment serves the cloud extension,
// returning [ExtensionNotAvailableError] when it does not.
//
// Every operation in this tree already checks, so calling it is never required.
// It exists for callers that want to fail early rather than partway through -
// a command-line tool checking before it starts work, say - and it shares the
// same cached probe, so asking first costs no extra round trip.
func (s CloudService) EnsureAvailable(ctx context.Context, opts ...option.RequestOption) error {
	return s.client.ensureCloudExtension(ctx, opts...)
}

// CloudAgentService groups the cloud agent operations.
type CloudAgentService struct {
	// Providers lists the agent providers registered on the deployment.
	Providers CloudAgentProviderService
}

// CloudCatalogService groups the built-in connector catalogs by connector kind,
// matching how the catalog is addressed on the wire.
type CloudCatalogService struct {
	Kafka   CloudCatalogKafkaService
	Sinks   CloudCatalogSinkService
	Sources CloudCatalogSourceService
}

// CloudConnectorService groups the connector runtimes.
type CloudConnectorService struct {
	Sinks   CloudConnectorSinkService
	Sources CloudConnectorSourceService
	Kafka   CloudConnectorKafkaService
}

// CloudAPIResourceService discovers the resources the cloud extension group
// advertises.
type CloudAPIResourceService struct {
	client *Client
}

// List returns the resources advertised by the cloud.sn.io/v1 API group.
func (s CloudAPIResourceService) List(ctx context.Context, opts ...option.RequestOption) (*APIResourceList, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	return s.client.GetCloudAPIResources(ctx, opts...)
}

// CloudAgentProviderService gates its operations on the cloud extension being available.
type CloudAgentProviderService struct {
	client *Client
	inner  *ProvidersClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into ProvidersClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudAgentProviderService) bind(ctx context.Context, opts []option.RequestOption) (*ProvidersClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewProvidersClient(client), nil
}

// List returns all managed-agent providers visible in the current workspace.
func (s CloudAgentProviderService) List(ctx context.Context, opts ...option.RequestOption) ([]AgentProviderInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []AgentProviderInfo
		return zero0, err
	}
	return inner.List(ctx)
}

// Get returns one managed-agent provider by name.
func (s CloudAgentProviderService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*AgentProviderInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *AgentProviderInfo
		return zero0, err
	}
	return inner.Get(ctx, name)
}

// CloudCatalogKafkaService gates its operations on the cloud extension being available.
type CloudCatalogKafkaService struct {
	client *Client
	inner  *CatalogClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into CatalogClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudCatalogKafkaService) bind(ctx context.Context, opts []option.RequestOption) (*CatalogClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewCatalogClient(client), nil
}

// List returns built-in Kafka Connect connector definitions.
func (s CloudCatalogKafkaService) List(ctx context.Context, opts ...option.RequestOption) ([]ConnectorDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConnectorDefinition
		return zero0, err
	}
	return inner.ListKafkaConnectors(ctx)
}

// Get returns configuration fields for a Kafka connector.
func (s CloudCatalogKafkaService) Get(ctx context.Context, name string, opts ...option.RequestOption) ([]ConfigFieldDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConfigFieldDefinition
		return zero0, err
	}
	return inner.GetKafkaConfigDefinition(ctx, name)
}

// CloudCatalogSinkService gates its operations on the cloud extension being available.
type CloudCatalogSinkService struct {
	client *Client
	inner  *CatalogClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into CatalogClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudCatalogSinkService) bind(ctx context.Context, opts []option.RequestOption) (*CatalogClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewCatalogClient(client), nil
}

// List returns all built-in sink connector definitions visible in the current workspace.
func (s CloudCatalogSinkService) List(ctx context.Context, opts ...option.RequestOption) ([]ConnectorDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConnectorDefinition
		return zero0, err
	}
	return inner.ListSinks(ctx)
}

// Get returns the configuration field definitions for a built-in sink connector.
func (s CloudCatalogSinkService) Get(ctx context.Context, name string, opts ...option.RequestOption) ([]ConfigFieldDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConfigFieldDefinition
		return zero0, err
	}
	return inner.GetSinkConfigDefinition(ctx, name)
}

// CloudCatalogSourceService gates its operations on the cloud extension being available.
type CloudCatalogSourceService struct {
	client *Client
	inner  *CatalogClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into CatalogClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudCatalogSourceService) bind(ctx context.Context, opts []option.RequestOption) (*CatalogClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewCatalogClient(client), nil
}

// List returns all built-in source connector definitions visible in the current workspace.
func (s CloudCatalogSourceService) List(ctx context.Context, opts ...option.RequestOption) ([]ConnectorDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConnectorDefinition
		return zero0, err
	}
	return inner.ListSources(ctx)
}

// Get returns the configuration field definitions for a built-in source connector.
func (s CloudCatalogSourceService) Get(ctx context.Context, name string, opts ...option.RequestOption) ([]ConfigFieldDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConfigFieldDefinition
		return zero0, err
	}
	return inner.GetSourceConfigDefinition(ctx, name)
}

// CloudConnectionService gates its operations on the cloud extension being available.
type CloudConnectionService struct {
	client *Client
	inner  *ConnectionsClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into ConnectionsClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudConnectionService) bind(ctx context.Context, opts []option.RequestOption) (*ConnectionsClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewConnectionsClient(client), nil
}

// List returns all connections visible in the current workspace.
func (s CloudConnectionService) List(ctx context.Context, opts ...option.RequestOption) ([]ConnectionConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConnectionConfig
		return zero0, err
	}
	return inner.List(ctx)
}

// Get returns one connection by name.
func (s CloudConnectionService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*ConnectionConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectionConfig
		return zero0, err
	}
	return inner.Get(ctx, name)
}

// Create creates a connection.
func (s CloudConnectionService) Create(ctx context.Context, cfg ConnectionConfig, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Create(ctx, cfg)
}

// Validate checks a connection configuration without creating it.
func (s CloudConnectionService) Validate(ctx context.Context, cfg ConnectionConfig, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Validate(ctx, cfg)
}

// Update updates a connection.
func (s CloudConnectionService) Update(ctx context.Context, name string, cfg ConnectionConfig, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Update(ctx, name, cfg)
}

// Delete deletes a connection by name.
func (s CloudConnectionService) Delete(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Delete(ctx, name)
}

// Test runs a connection health test.
func (s CloudConnectionService) Test(ctx context.Context, name string, opts ...option.RequestOption) (*ConnectionHealthStatus, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectionHealthStatus
		return zero0, err
	}
	return inner.Test(ctx, name)
}

// CloudConnectorSinkService gates its operations on the cloud extension being available.
type CloudConnectorSinkService struct {
	client *Client
	inner  *SinksClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into SinksClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudConnectorSinkService) bind(ctx context.Context, opts []option.RequestOption) (*SinksClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewSinksClient(client), nil
}

// List returns all sink names visible in the current workspace.
func (s CloudConnectorSinkService) List(ctx context.Context, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.List(ctx)
}

// Get returns one sink config by name.
func (s CloudConnectorSinkService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*RegistrySinkConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *RegistrySinkConfig
		return zero0, err
	}
	return inner.Get(ctx, name)
}

// Create creates a sink using multipart/form-data.
func (s CloudConnectorSinkService) Create(ctx context.Context, cfg RegistrySinkConfig, filePath, packageURL string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Create(ctx, cfg, filePath, packageURL)
}

// Update updates a sink using multipart/form-data.
func (s CloudConnectorSinkService) Update(ctx context.Context, name string, cfg RegistrySinkConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Update(ctx, name, cfg, filePath, packageURL, updateOptions)
}

// Delete deletes a sink by name.
func (s CloudConnectorSinkService) Delete(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Delete(ctx, name)
}

// Start starts all instances for a sink.
func (s CloudConnectorSinkService) Start(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Start(ctx, name)
}

// Stop stops all instances for a sink.
func (s CloudConnectorSinkService) Stop(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Stop(ctx, name)
}

// Restart restarts all instances for a sink.
func (s CloudConnectorSinkService) Restart(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Restart(ctx, name)
}

// StartInstance starts one sink instance.
func (s CloudConnectorSinkService) StartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StartInstance(ctx, name, instanceID)
}

// StopInstance stops one sink instance.
func (s CloudConnectorSinkService) StopInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StopInstance(ctx, name, instanceID)
}

// RestartInstance restarts one sink instance.
func (s CloudConnectorSinkService) RestartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.RestartInstance(ctx, name, instanceID)
}

// Status returns the aggregate sink status.
func (s CloudConnectorSinkService) Status(ctx context.Context, name string, opts ...option.RequestOption) (*SinkStatus, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *SinkStatus
		return zero0, err
	}
	return inner.Status(ctx, name)
}

// InstanceStatus returns one sink instance status.
func (s CloudConnectorSinkService) InstanceStatus(ctx context.Context, name, instanceID string, opts ...option.RequestOption) (*SinkInstanceStatusData, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *SinkInstanceStatusData
		return zero0, err
	}
	return inner.InstanceStatus(ctx, name, instanceID)
}

// CloudConnectorSourceService gates its operations on the cloud extension being available.
type CloudConnectorSourceService struct {
	client *Client
	inner  *SourcesClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into SourcesClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudConnectorSourceService) bind(ctx context.Context, opts []option.RequestOption) (*SourcesClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewSourcesClient(client), nil
}

// List returns all source names visible in the current workspace.
func (s CloudConnectorSourceService) List(ctx context.Context, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.List(ctx)
}

// Get returns one source config by name.
func (s CloudConnectorSourceService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*RegistrySourceConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *RegistrySourceConfig
		return zero0, err
	}
	return inner.Get(ctx, name)
}

// Create creates a source using multipart/form-data.
func (s CloudConnectorSourceService) Create(ctx context.Context, cfg RegistrySourceConfig, filePath, packageURL string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Create(ctx, cfg, filePath, packageURL)
}

// Update updates a source using multipart/form-data.
func (s CloudConnectorSourceService) Update(ctx context.Context, name string, cfg RegistrySourceConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Update(ctx, name, cfg, filePath, packageURL, updateOptions)
}

// Delete deletes a source by name.
func (s CloudConnectorSourceService) Delete(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Delete(ctx, name)
}

// Start starts all instances for a source.
func (s CloudConnectorSourceService) Start(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Start(ctx, name)
}

// Stop stops all instances for a source.
func (s CloudConnectorSourceService) Stop(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Stop(ctx, name)
}

// Restart restarts all instances for a source.
func (s CloudConnectorSourceService) Restart(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Restart(ctx, name)
}

// StartInstance starts one source instance.
func (s CloudConnectorSourceService) StartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StartInstance(ctx, name, instanceID)
}

// StopInstance stops one source instance.
func (s CloudConnectorSourceService) StopInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StopInstance(ctx, name, instanceID)
}

// RestartInstance restarts one source instance.
func (s CloudConnectorSourceService) RestartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.RestartInstance(ctx, name, instanceID)
}

// Status returns the aggregate source status.
func (s CloudConnectorSourceService) Status(ctx context.Context, name string, opts ...option.RequestOption) (*SourceStatus, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *SourceStatus
		return zero0, err
	}
	return inner.Status(ctx, name)
}

// InstanceStatus returns one source instance status.
func (s CloudConnectorSourceService) InstanceStatus(ctx context.Context, name, instanceID string, opts ...option.RequestOption) (*SourceInstanceStatusData, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *SourceInstanceStatusData
		return zero0, err
	}
	return inner.InstanceStatus(ctx, name, instanceID)
}

// CloudConnectorKafkaService gates its operations on the cloud extension being available.
type CloudConnectorKafkaService struct {
	client *Client
	inner  *KafkaConnectClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into KafkaConnectClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudConnectorKafkaService) bind(ctx context.Context, opts []option.RequestOption) (*KafkaConnectClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewKafkaConnectClient(client), nil
}

// GetInfo returns Kafka Connect server info.
func (s CloudConnectorKafkaService) GetInfo(ctx context.Context, opts ...option.RequestOption) (*ServerInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ServerInfo
		return zero0, err
	}
	return inner.GetInfo(ctx)
}

// GetHealth returns Kafka Connect worker health.
func (s CloudConnectorKafkaService) GetHealth(ctx context.Context, opts ...option.RequestOption) (*WorkerStatus, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *WorkerStatus
		return zero0, err
	}
	return inner.GetHealth(ctx)
}

// ListConnectors lists connector names.
func (s CloudConnectorKafkaService) ListConnectors(ctx context.Context, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.ListConnectors(ctx)
}

// GetConnector returns one connector by name.
func (s CloudConnectorKafkaService) GetConnector(ctx context.Context, name string, opts ...option.RequestOption) (*ConnectorInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorInfo
		return zero0, err
	}
	return inner.GetConnector(ctx, name)
}

// CreateConnector creates a new connector and returns the server response.
func (s CloudConnectorKafkaService) CreateConnector(ctx context.Context, req CreateConnectorRequest, opts ...option.RequestOption) (*ConnectorInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorInfo
		return zero0, err
	}
	return inner.CreateConnector(ctx, req)
}

// UpdateConnectorConfig replaces connector config and returns the server response.
func (s CloudConnectorKafkaService) UpdateConnectorConfig(ctx context.Context, name string, config map[string]string, opts ...option.RequestOption) (*ConnectorInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorInfo
		return zero0, err
	}
	return inner.UpdateConnectorConfig(ctx, name, config)
}

// DeleteConnector deletes a connector.
func (s CloudConnectorKafkaService) DeleteConnector(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.DeleteConnector(ctx, name)
}

// PauseConnector pauses a connector.
func (s CloudConnectorKafkaService) PauseConnector(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.PauseConnector(ctx, name)
}

// ResumeConnector resumes a connector.
func (s CloudConnectorKafkaService) ResumeConnector(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.ResumeConnector(ctx, name)
}

// RestartConnector restarts only the connector, preserving the previous simple API.
func (s CloudConnectorKafkaService) RestartConnector(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.RestartConnector(ctx, name)
}

// RestartConnectorWithOptions restarts a connector and optionally its tasks.
func (s CloudConnectorKafkaService) RestartConnectorWithOptions(ctx context.Context, name string, options RestartConnectorOptions, opts ...option.RequestOption) (*ConnectorStateInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorStateInfo
		return zero0, err
	}
	return inner.RestartConnectorWithOptions(ctx, name, options)
}

// StopConnector stops a connector.
func (s CloudConnectorKafkaService) StopConnector(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StopConnector(ctx, name)
}

// GetConnectorConfig returns the connector config map.
func (s CloudConnectorKafkaService) GetConnectorConfig(ctx context.Context, name string, opts ...option.RequestOption) (map[string]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 map[string]string
		return zero0, err
	}
	return inner.GetConnectorConfig(ctx, name)
}

// GetConnectorStatus returns aggregate connector status.
func (s CloudConnectorKafkaService) GetConnectorStatus(ctx context.Context, name string, opts ...option.RequestOption) (*ConnectorStateInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorStateInfo
		return zero0, err
	}
	return inner.GetConnectorStatus(ctx, name)
}

// GetConnectorTasks returns connector task configurations.
func (s CloudConnectorKafkaService) GetConnectorTasks(ctx context.Context, name string, opts ...option.RequestOption) ([]TaskConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []TaskConfig
		return zero0, err
	}
	return inner.GetConnectorTasks(ctx, name)
}

// GetConnectorTasksConfig returns the deprecated task-config map.
func (s CloudConnectorKafkaService) GetConnectorTasksConfig(ctx context.Context, name string, opts ...option.RequestOption) (map[string]map[string]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 map[string]map[string]string
		return zero0, err
	}
	return inner.GetConnectorTasksConfig(ctx, name)
}

// GetTaskStatus returns one connector task status.
func (s CloudConnectorKafkaService) GetTaskStatus(ctx context.Context, connector string, taskID int, opts ...option.RequestOption) (*TaskState, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *TaskState
		return zero0, err
	}
	return inner.GetTaskStatus(ctx, connector, taskID)
}

// RestartTask restarts one connector task.
func (s CloudConnectorKafkaService) RestartTask(ctx context.Context, connector string, taskID int, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.RestartTask(ctx, connector, taskID)
}

// DescribePluginConfig returns plugin configuration definitions.
func (s CloudConnectorKafkaService) DescribePluginConfig(ctx context.Context, pluginName string, opts ...option.RequestOption) ([]ConfigKeyInfo, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []ConfigKeyInfo
		return zero0, err
	}
	return inner.DescribePluginConfig(ctx, pluginName)
}

// ListPluginCatalog returns installed Function Mesh connector definitions.
func (s CloudConnectorKafkaService) ListPluginCatalog(ctx context.Context, opts ...option.RequestOption) ([]FunctionMeshConnectorDefinition, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []FunctionMeshConnectorDefinition
		return zero0, err
	}
	return inner.ListPluginCatalog(ctx)
}

// GetActiveTopics returns topics actively used by a connector.
func (s CloudConnectorKafkaService) GetActiveTopics(ctx context.Context, connector string, opts ...option.RequestOption) (ConnectorActiveTopics, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 ConnectorActiveTopics
		return zero0, err
	}
	return inner.GetActiveTopics(ctx, connector)
}

// ResetActiveTopics clears a connector's active-topic tracking data.
func (s CloudConnectorKafkaService) ResetActiveTopics(ctx context.Context, connector string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.ResetActiveTopics(ctx, connector)
}

// GetOffsets returns current connector offsets.
func (s CloudConnectorKafkaService) GetOffsets(ctx context.Context, connector string, opts ...option.RequestOption) (*ConnectorOffsets, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *ConnectorOffsets
		return zero0, err
	}
	return inner.GetOffsets(ctx, connector)
}

// ResetOffsets resets connector offsets.
func (s CloudConnectorKafkaService) ResetOffsets(ctx context.Context, connector string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.ResetOffsets(ctx, connector)
}

// AlterOffsets alters connector offsets.
func (s CloudConnectorKafkaService) AlterOffsets(ctx context.Context, connector string, offsets ConnectorOffsets, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.AlterOffsets(ctx, connector, offsets)
}

// CloudFunctionService gates its operations on the cloud extension being available.
type CloudFunctionService struct {
	client *Client
	inner  *FunctionsClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into FunctionsClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudFunctionService) bind(ctx context.Context, opts []option.RequestOption) (*FunctionsClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewFunctionsClient(client), nil
}

// List returns all function names visible in the current workspace.
func (s CloudFunctionService) List(ctx context.Context, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.List(ctx)
}

// Get returns one function config by name.
func (s CloudFunctionService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*RegistryFunctionConfig, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *RegistryFunctionConfig
		return zero0, err
	}
	return inner.Get(ctx, name)
}

// Create creates a function using multipart/form-data.
func (s CloudFunctionService) Create(ctx context.Context, cfg RegistryFunctionConfig, filePath, packageURL string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Create(ctx, cfg, filePath, packageURL)
}

// Update updates a function using multipart/form-data.
func (s CloudFunctionService) Update(ctx context.Context, name string, cfg RegistryFunctionConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Update(ctx, name, cfg, filePath, packageURL, updateOptions)
}

// Delete deletes a function by name.
func (s CloudFunctionService) Delete(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Delete(ctx, name)
}

// Start starts all instances for a function.
func (s CloudFunctionService) Start(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Start(ctx, name)
}

// Stop stops all instances for a function.
func (s CloudFunctionService) Stop(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Stop(ctx, name)
}

// Restart restarts all instances for a function.
func (s CloudFunctionService) Restart(ctx context.Context, name string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Restart(ctx, name)
}

// StartInstance starts one function instance.
func (s CloudFunctionService) StartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StartInstance(ctx, name, instanceID)
}

// StopInstance stops one function instance.
func (s CloudFunctionService) StopInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.StopInstance(ctx, name, instanceID)
}

// RestartInstance restarts one function instance.
func (s CloudFunctionService) RestartInstance(ctx context.Context, name, instanceID string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.RestartInstance(ctx, name, instanceID)
}

// Status returns the aggregate function status.
func (s CloudFunctionService) Status(ctx context.Context, name string, opts ...option.RequestOption) (*FunctionStatus, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *FunctionStatus
		return zero0, err
	}
	return inner.Status(ctx, name)
}

// InstanceStatus returns one function instance status.
func (s CloudFunctionService) InstanceStatus(ctx context.Context, name, instanceID string, opts ...option.RequestOption) (*FunctionInstanceStatusData, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *FunctionInstanceStatusData
		return zero0, err
	}
	return inner.InstanceStatus(ctx, name, instanceID)
}

// Stats returns aggregate and per-instance function metrics.
func (s CloudFunctionService) Stats(ctx context.Context, name string, opts ...option.RequestOption) (*FunctionStats, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *FunctionStats
		return zero0, err
	}
	return inner.Stats(ctx, name)
}

// InstanceStats returns metrics for one function instance.
func (s CloudFunctionService) InstanceStats(ctx context.Context, name, instanceID string, opts ...option.RequestOption) (*FunctionInstanceStatsData, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *FunctionInstanceStatsData
		return zero0, err
	}
	return inner.InstanceStats(ctx, name, instanceID)
}

// Trigger invokes a function with inline data or a data stream file.
func (s CloudFunctionService) Trigger(ctx context.Context, name, data, dataFilePath, topic string, opts ...option.RequestOption) (string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 string
		return zero0, err
	}
	return inner.Trigger(ctx, name, data, dataFilePath, topic)
}

// GetState returns one function state value.
func (s CloudFunctionService) GetState(ctx context.Context, name, key string, opts ...option.RequestOption) (*FunctionState, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *FunctionState
		return zero0, err
	}
	return inner.GetState(ctx, name, key)
}

// PutState updates one function state value.
func (s CloudFunctionService) PutState(ctx context.Context, name, key string, state FunctionState, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.PutState(ctx, name, key, state)
}

// CloudHealthService gates its operations on the cloud extension being available.
type CloudHealthService struct {
	client *Client
	inner  *HealthClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into HealthClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudHealthService) bind(ctx context.Context, opts []option.RequestOption) (*HealthClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewHealthClient(client), nil
}

// Health reports aggregate registry health.
func (s CloudHealthService) Health(ctx context.Context, opts ...option.RequestOption) (bool, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 bool
		return zero0, err
	}
	return inner.Health(ctx)
}

// Ready reports registry readiness.
func (s CloudHealthService) Ready(ctx context.Context, opts ...option.RequestOption) (bool, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 bool
		return zero0, err
	}
	return inner.Ready(ctx)
}

// Live reports registry liveness.
func (s CloudHealthService) Live(ctx context.Context, opts ...option.RequestOption) (bool, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 bool
		return zero0, err
	}
	return inner.Live(ctx)
}

// get calls the underlying cloud extension operation.
func (s CloudHealthService) get(ctx context.Context, path string, opts ...option.RequestOption) (bool, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 bool
		return zero0, err
	}
	return inner.get(ctx, path)
}

// CloudPackageService gates its operations on the cloud extension being available.
type CloudPackageService struct {
	client *Client
	inner  *PackagesClient
}

// bind gates the call and returns the client to run it on. Per-call
// options are applied by scoping a fresh client rather than being
// threaded into PackagesClient, whose signatures consumers implement
// interfaces against - adding a parameter there would break them.
func (s CloudPackageService) bind(ctx context.Context, opts []option.RequestOption) (*PackagesClient, error) {
	if err := s.client.ensureCloudExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return s.inner, nil
	}
	client, err := s.client.With(opts...)
	if err != nil {
		return nil, err
	}
	return NewPackagesClient(client), nil
}

// List returns all package names for the given package type.
func (s CloudPackageService) List(ctx context.Context, packageType string, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.List(ctx, packageType)
}

// ListVersions returns all versions for the given package.
func (s CloudPackageService) ListVersions(ctx context.Context, packageType, packageName string, opts ...option.RequestOption) ([]string, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 []string
		return zero0, err
	}
	return inner.ListVersions(ctx, packageType, packageName)
}

// GetMetadata returns the metadata for one package version.
func (s CloudPackageService) GetMetadata(ctx context.Context, packageType, packageName, version string, opts ...option.RequestOption) (*PackageMetadata, error) {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		var zero0 *PackageMetadata
		return zero0, err
	}
	return inner.GetMetadata(ctx, packageType, packageName, version)
}

// UpdateMetadata updates metadata for one package version.
func (s CloudPackageService) UpdateMetadata(ctx context.Context, packageType, packageName, version string, metadata PackageMetadata, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.UpdateMetadata(ctx, packageType, packageName, version, metadata)
}

// Upload uploads one package version via multipart/form-data.
func (s CloudPackageService) Upload(ctx context.Context, packageType, packageName, version, filePath string, metadata PackageMetadata, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Upload(ctx, packageType, packageName, version, filePath, metadata)
}

// Download streams one package version into the provided writer.
func (s CloudPackageService) Download(ctx context.Context, packageType, packageName, version string, writer io.Writer, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Download(ctx, packageType, packageName, version, writer)
}

// Delete removes one package version.
func (s CloudPackageService) Delete(ctx context.Context, packageType, packageName, version string, opts ...option.RequestOption) error {
	inner, err := s.bind(ctx, opts)
	if err != nil {
		return err
	}
	return inner.Delete(ctx, packageType, packageName, version)
}
