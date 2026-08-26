// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Ported from orca-sdk-typescript tests/cloud-extensions-contract.test.ts.
//
// The TypeScript test regex-extracts every `operationId:` out of
// openapi/cloud-extensions.yaml and asserts the sorted list equals a
// hand-maintained map of SDK accessors, then asserts each accessor is a
// function. Go cannot look an accessor up by name at run time, so the port
// trades reflection for something stronger: every operation is mapped to a real
// call that must issue exactly the method and path the spec declares for it.
//
// Two properties are asserted:
//
//  1. Coverage - every operationId in the vendored spec is either mapped by
//     cloudContractOperations or listed in cloudExcludedOperations with a reason.
//  2. Conformance - invoking each mapping produces one request whose method
//     equals the spec's method and whose path matches the spec's path template,
//     rooted at the literal cloud extension prefix.
//
// The operation table itself is assembled from the per-resource tables in the
// sibling cloud_*_port_test.go files, so there is exactly one definition of how
// each operation is called.

// cloudExtensionPathPrefix is the prefix every cloud extension request must carry.
// It is spelled out literally rather than built from CloudExtensionBasePath: a
// test that reused the implementation's own constant would keep passing if the
// constant itself were wrong.
const cloudExtensionPathPrefix = "/apis/cloud.sn.io/v1"

// cloudOperation is one spec operation, the request it must produce, and the Go
// call that produces it. It mirrors the {name, method, path, invoke} shape the
// TypeScript cloud tests use.
type cloudOperation struct {
	// operationID is the operationId in openapi/cloud-extensions.yaml.
	operationID string
	// name is the subtest name, matching the TypeScript case name.
	name string
	// method and path are the full request this operation must issue. path is
	// percent-encoded, so a connection named "events/primary" appears as
	// "events%2Fprimary".
	method string
	path   string
	invoke func(ctx context.Context, client *Client) error
}

// cloudSpecOperation is one row of the vendored OpenAPI contract.
type cloudSpecOperation struct {
	operationID string
	method      string
	// path is the spec path template, relative to the cloud extension prefix,
	// with {placeholders} for path parameters.
	path string
}

// cloudSpecOperations is every operationId in
// orca-sdk-typescript/openapi/cloud-extensions.yaml (96 of them), with the
// method and path template the spec declares. This SDK does not vendor the
// spec, so the table is transcribed here;
// TestCloudExtensionContractSpecTableMatchesVendoredOpenAPI re-derives it from
// the YAML whenever a checkout of it can be found, which is what keeps the
// transcription honest.
var cloudSpecOperations = []cloudSpecOperation{
	{"alterConnectorOffsets", "PATCH", "/connectors/kafka/connectors/{connector}/offsets"},
	{"createConnection", "POST", "/connections"},
	{"createConnector", "POST", "/connectors/kafka/connectors"},
	{"delete", "DELETE", "/packages/{type}/{packageName}/{version}"},
	{"deleteConnection", "DELETE", "/connections/{name}"},
	{"deregisterFunctionWithDefaults", "DELETE", "/functions/{functionName}"},
	{"deregisterSinkWithDefaults", "DELETE", "/connectors/sinks/{sinkName}"},
	{"deregisterSourceWithDefaults", "DELETE", "/connectors/sources/{sourceName}"},
	{"destroyConnector", "DELETE", "/connectors/kafka/connectors/{connector}"},
	{"download", "GET", "/packages/{type}/{packageName}/{version}"},
	{"getApiResources", "GET", "/"},
	{"getConnection", "GET", "/connections/{name}"},
	{"getConnector", "GET", "/connectors/kafka/connectors/{connector}"},
	{"getConnectorActiveTopics", "GET", "/connectors/kafka/connectors/{connector}/topics"},
	{"getConnectorConfig", "GET", "/connectors/kafka/connectors/{connector}/config"},
	{"getConnectorConfigDef", "GET", "/connectors/kafka/connector-plugins/{pluginName}/config"},
	{"getConnectorStatus", "GET", "/connectors/kafka/connectors/{connector}/status"},
	{"getFunctionInfoWithDefaults", "GET", "/functions/{functionName}"},
	{"getFunctionInstanceStatsWithDefaults", "GET", "/functions/{functionName}/{instanceId}/stats"},
	{"getFunctionInstanceStatusWithDefaults", "GET", "/functions/{functionName}/{instanceId}/status"},
	{"getFunctionStateWithDefaults", "GET", "/functions/{functionName}/state/{key}"},
	{"getFunctionStatsWithDefaults", "GET", "/functions/{functionName}/stats"},
	{"getFunctionStatusWithDefaults", "GET", "/functions/{functionName}/status"},
	{"getKafkaConnectorConfigDefinition", "GET", "/catalog/kafka/{name}"},
	{"getKafkaConnectorList", "GET", "/catalog/kafka"},
	{"getMeta", "GET", "/packages/{type}/{packageName}/{version}/metadata"},
	{"getOffsets", "GET", "/connectors/kafka/connectors/{connector}/offsets"},
	{"getProvider", "GET", "/agents/providers/{providerName}"},
	{"getSinkConfigDefinition", "GET", "/catalog/sinks/{name}"},
	{"getSinkInfoWithDefaults", "GET", "/connectors/sinks/{sinkName}"},
	{"getSinkInstanceStatusWithDefaults", "GET", "/connectors/sinks/{sinkName}/{instanceId}/status"},
	{"getSinkList", "GET", "/catalog/sinks"},
	{"getSinkStatusWithDefaults", "GET", "/connectors/sinks/{sinkName}/status"},
	{"getSourceConfigDefinition", "GET", "/catalog/sources/{name}"},
	{"getSourceInfoWithDefaults", "GET", "/connectors/sources/{sourceName}"},
	{"getSourceInstanceStatusWithDefaults", "GET", "/connectors/sources/{sourceName}/{instanceId}/status"},
	{"getSourceList", "GET", "/catalog/sources"},
	{"getSourceStatusWithDefaults", "GET", "/connectors/sources/{sourceName}/status"},
	{"getTaskConfigs", "GET", "/connectors/kafka/connectors/{connector}/tasks"},
	{"getTaskStatus", "GET", "/connectors/kafka/connectors/{connector}/tasks/{task}/status"},
	{"getTasksConfig", "GET", "/connectors/kafka/connectors/{connector}/tasks-config"},
	{"health", "GET", "/health"},
	{"healthCheck", "GET", "/connectors/kafka/health"},
	{"isInitialized", "GET", "/health/ready"},
	{"listConnections", "GET", "/connections"},
	{"listConnectorPlugins", "GET", "/connectors/kafka/connector-plugins"},
	{"listConnectorPluginsCatalog", "GET", "/connectors/kafka/connector-plugins/catalog"},
	{"listConnectors", "GET", "/connectors/kafka/connectors"},
	{"listFunctionsWithDefaults", "GET", "/functions"},
	{"listPackageVersion", "GET", "/packages/{type}/{packageName}"},
	{"listPackages", "GET", "/packages/{type}"},
	{"listProviders", "GET", "/agents/providers"},
	{"listSinkWithDefaults", "GET", "/connectors/sinks"},
	{"listSourcesWithDefaults", "GET", "/connectors/sources"},
	{"liveness", "GET", "/health/live"},
	{"pauseConnector", "PUT", "/connectors/kafka/connectors/{connector}:pause"},
	{"putConnectorConfig", "PUT", "/connectors/kafka/connectors/{connector}/config"},
	{"putFunctionStateWithDefaults", "POST", "/functions/{functionName}/state/{key}"},
	{"registerFunctionWithDefaults", "POST", "/functions/{functionName}"},
	{"registerSinkWithDefaults", "POST", "/connectors/sinks/{sinkName}"},
	{"registerSourceWithDefaults", "POST", "/connectors/sources/{sourceName}"},
	{"resetConnectorActiveTopics", "PUT", "/connectors/kafka/connectors/{connector}/topics:reset"},
	{"resetConnectorOffsets", "DELETE", "/connectors/kafka/connectors/{connector}/offsets"},
	{"restartConnector", "POST", "/connectors/kafka/connectors/{connector}:restart"},
	{"restartFunctionAllWithDefaults", "POST", "/functions/{functionName}:restart"},
	{"restartFunctionWithDefaults", "POST", "/functions/{functionName}/{instanceId}:restart"},
	{"restartSinkAllWithDefaults", "POST", "/connectors/sinks/{sinkName}:restart"},
	{"restartSinkWithDefaults", "POST", "/connectors/sinks/{sinkName}/{instanceId}:restart"},
	{"restartSourceAllWithDefaults", "POST", "/connectors/sources/{sourceName}:restart"},
	{"restartSourceWithDefaults", "POST", "/connectors/sources/{sourceName}/{instanceId}:restart"},
	{"restartTask", "POST", "/connectors/kafka/connectors/{connector}/tasks/{task}/restart"},
	{"resumeConnector", "PUT", "/connectors/kafka/connectors/{connector}:resume"},
	{"serverInfo", "GET", "/connectors/kafka"},
	{"startFunctionAllWithDefaults", "POST", "/functions/{functionName}:start"},
	{"startFunctionWithDefaults", "POST", "/functions/{functionName}/{instanceId}:start"},
	{"startSinkAllWithDefaults", "POST", "/connectors/sinks/{sinkName}:start"},
	{"startSinkWithDefaults", "POST", "/connectors/sinks/{sinkName}/{instanceId}:start"},
	{"startSourceAllWithDefaults", "POST", "/connectors/sources/{sourceName}:start"},
	{"startSourceWithDefaults", "POST", "/connectors/sources/{sourceName}/{instanceId}:start"},
	{"stopConnector", "PUT", "/connectors/kafka/connectors/{connector}:stop"},
	{"stopFunctionAllWithDefaults", "POST", "/functions/{functionName}:stop"},
	{"stopFunctionWithDefaults", "POST", "/functions/{functionName}/{instanceId}:stop"},
	{"stopSinkAllWithDefaults", "POST", "/connectors/sinks/{sinkName}:stop"},
	{"stopSinkWithDefaults", "POST", "/connectors/sinks/{sinkName}/{instanceId}:stop"},
	{"stopSourceAllWithDefaults", "POST", "/connectors/sources/{sourceName}:stop"},
	{"stopSourceWithDefaults", "POST", "/connectors/sources/{sourceName}/{instanceId}:stop"},
	{"testConnection", "GET", "/connections/{name}:test"},
	{"triggerFunctionWithDefaults", "POST", "/functions/{functionName}:trigger"},
	{"updateConnection", "PUT", "/connections/{name}"},
	{"updateFunctionWithDefaults", "PUT", "/functions/{functionName}"},
	{"updateMeta", "PUT", "/packages/{type}/{packageName}/{version}/metadata"},
	{"updateSinkWithDefaults", "PUT", "/connectors/sinks/{sinkName}"},
	{"updateSourceWithDefaults", "PUT", "/connectors/sources/{sourceName}"},
	{"upload", "POST", "/packages/{type}/{packageName}/{version}"},
	{"validateConfigs", "PUT", "/connectors/kafka/connector-plugins/{pluginName}/config/validate"},
	{"validateConnection", "POST", "/connections/validate"},
}

// cloudExcludedOperations are spec operations that are deliberately not mapped
// to a request-issuing call, each with the reason. The TypeScript suite drops
// the same single operation from its accessor map.
var cloudExcludedOperations = map[string]string{
	"validateConfigs": "the server's only declared response for it is HTTP 400; " +
		"KafkaConnectClient.ValidateConfig therefore fails locally with " +
		"ErrKafkaConnectConfigValidationUnsupported instead of issuing a request",
}

// cloudContractOperations is the union of the per-resource operation tables.
func cloudContractOperations() []cloudOperation {
	var operations []cloudOperation
	operations = append(operations, cloudCatalogOperations()...)
	operations = append(operations, cloudConnectionOperations()...)
	operations = append(operations, cloudConnectorOperations()...)
	operations = append(operations, cloudFunctionOperations()...)
	operations = append(operations, cloudPackageOperations()...)
	return operations
}

// cloudTestClient returns a bearer client whose every request is captured and
// answered with a JSON `null` body. `null` decodes without error into every
// result type this SDK returns (slices and maps become nil, scalars and structs
// keep their zero value), so one responder serves the whole surface and the
// tests stay about the request that went out.
func cloudTestClient(tb testing.TB) (*Client, *recordingTransport) {
	tb.Helper()
	return newRecordingClient(tb, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "null"), nil
	})
}

// cloudAssertOperation invokes one operation and asserts it issued exactly the
// request it declares, carrying the caller's credential.
//
// The TypeScript original also asserts a per-request `X-Test-Header` survives
// into the request. This SDK has no per-request options argument, so the
// authorization header - the one header it does attach - stands in for that.
func cloudAssertOperation(t *testing.T, operation cloudOperation) {
	t.Helper()

	client, transport := cloudTestClient(t)
	if err := operation.invoke(context.Background(), client); err != nil {
		t.Fatalf("invoke() error = %v", err)
	}

	call := transport.Only(t)
	if call.Method != operation.method {
		t.Errorf("method = %q, want %q", call.Method, operation.method)
	}
	if got := call.Path(); got != operation.path {
		t.Errorf("path = %q, want %q", got, operation.path)
	}
	if got := call.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
	}
}

// cloudRunOperations is the shared body of every per-resource table test.
func cloudRunOperations(t *testing.T, operations []cloudOperation) {
	t.Helper()
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			cloudAssertOperation(t, operation)
		})
	}
}

// cloudSpecPathPattern turns a spec path template into an anchored pattern for
// the full request path. Each {placeholder} stands for exactly one path
// segment, which is what percent-encoding a name containing "/" guarantees.
func cloudSpecPathPattern(tb testing.TB, template string) *regexp.Regexp {
	tb.Helper()

	var pattern strings.Builder
	pattern.WriteString("^" + regexp.QuoteMeta(cloudExtensionPathPrefix))
	for len(template) > 0 {
		open := strings.IndexByte(template, '{')
		if open < 0 {
			pattern.WriteString(regexp.QuoteMeta(template))
			break
		}
		closing := strings.IndexByte(template[open:], '}')
		if closing < 0 {
			tb.Fatalf("spec path template %q has an unterminated placeholder", template)
		}
		pattern.WriteString(regexp.QuoteMeta(template[:open]))
		pattern.WriteString("[^/]+")
		template = template[open+closing+1:]
	}
	pattern.WriteString("$")

	return regexp.MustCompile(pattern.String())
}

func TestCloudExtensionContractCoversEverySpecOperation(t *testing.T) {
	t.Parallel()

	mapped := map[string][]cloudOperation{}
	for _, operation := range cloudContractOperations() {
		mapped[operation.operationID] = append(mapped[operation.operationID], operation)
	}

	var uncovered, excluded []string
	for _, spec := range cloudSpecOperations {
		switch {
		case len(mapped[spec.operationID]) > 0:
		case cloudExcludedOperations[spec.operationID] != "":
			excluded = append(excluded, spec.operationID)
		default:
			uncovered = append(uncovered, spec.operationID)
		}
	}
	sort.Strings(uncovered)
	sort.Strings(excluded)

	covered := len(cloudSpecOperations) - len(uncovered) - len(excluded)
	t.Logf("cloud extension coverage: %d of %d spec operations (%d documented exclusions: %s)",
		covered, len(cloudSpecOperations), len(excluded), strings.Join(excluded, ", "))

	if len(uncovered) > 0 {
		t.Errorf("%d spec operations have no mapped client call: %s",
			len(uncovered), strings.Join(uncovered, ", "))
	}

	// Every mapping must correspond to a real spec operation, and no operation
	// may be claimed twice - a duplicate would inflate the coverage count.
	known := map[string]bool{}
	for _, spec := range cloudSpecOperations {
		known[spec.operationID] = true
	}
	for operationID, operations := range mapped {
		if !known[operationID] {
			t.Errorf("operation %q is mapped but is not an operationId in the spec", operationID)
		}
		if len(operations) > 1 {
			names := make([]string, 0, len(operations))
			for _, operation := range operations {
				names = append(names, operation.name)
			}
			sort.Strings(names)
			t.Errorf("operation %q is mapped %d times (%s); each must be mapped once",
				operationID, len(operations), strings.Join(names, ", "))
		}
	}
}

func TestCloudExtensionContractOperationsIssueTheirSpecRequest(t *testing.T) {
	t.Parallel()

	specs := map[string]cloudSpecOperation{}
	for _, spec := range cloudSpecOperations {
		specs[spec.operationID] = spec
	}

	for _, operation := range cloudContractOperations() {
		t.Run(operation.operationID, func(t *testing.T) {
			t.Parallel()

			spec, ok := specs[operation.operationID]
			if !ok {
				t.Fatalf("no spec operation named %q", operation.operationID)
			}

			client, transport := cloudTestClient(t)
			if err := operation.invoke(context.Background(), client); err != nil {
				t.Fatalf("invoke() error = %v", err)
			}

			call := transport.Only(t)
			if call.Method != spec.method {
				t.Errorf("method = %q, want the spec's %q", call.Method, spec.method)
			}
			if pattern := cloudSpecPathPattern(t, spec.path); !pattern.MatchString(call.Path()) {
				t.Errorf("path = %q, want a match for the spec path %q (%s)",
					call.Path(), spec.path, pattern)
			}
		})
	}
}

// TestCloudExtensionContractSpecTableMatchesVendoredOpenAPI re-derives the
// operationId list from the vendored spec, exactly as the TypeScript test does,
// whenever a checkout of orca-sdk-typescript is reachable. It is the guard
// against cloudSpecOperations drifting from the contract it transcribes.
func TestCloudExtensionContractSpecTableMatchesVendoredOpenAPI(t *testing.T) {
	t.Parallel()

	candidates := []string{
		os.Getenv("ORCA_CLOUD_EXTENSIONS_SPEC"),
		"../orca-sdk-typescript/openapi/cloud-extensions.yaml",
	}
	var spec []byte
	var source string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		contents, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		spec, source = contents, candidate
		break
	}
	if spec == nil {
		t.Skip("vendored openapi/cloud-extensions.yaml not found; " +
			"set ORCA_CLOUD_EXTENSIONS_SPEC to cross-check cloudSpecOperations")
	}

	operationIDPattern := regexp.MustCompile(`(?m)^\s+operationId:\s+(\S+)$`)
	var fromSpec []string
	for _, match := range operationIDPattern.FindAllStringSubmatch(string(spec), -1) {
		fromSpec = append(fromSpec, match[1])
	}
	sort.Strings(fromSpec)

	transcribed := make([]string, 0, len(cloudSpecOperations))
	for _, operation := range cloudSpecOperations {
		transcribed = append(transcribed, operation.operationID)
	}
	sort.Strings(transcribed)

	if strings.Join(fromSpec, "\n") != strings.Join(transcribed, "\n") {
		t.Errorf("cloudSpecOperations has drifted from %s:\nspec has %d operationIds, table has %d\n%s",
			source, len(fromSpec), len(transcribed), cloudDiffStrings(transcribed, fromSpec))
	}
}

// cloudDiffStrings renders the symmetric difference of two sorted lists.
func cloudDiffStrings(have, want []string) string {
	inWant := map[string]bool{}
	for _, value := range want {
		inWant[value] = true
	}
	inHave := map[string]bool{}
	for _, value := range have {
		inHave[value] = true
	}

	var lines []string
	for _, value := range have {
		if !inWant[value] {
			lines = append(lines, "  only in the table: "+value)
		}
	}
	for _, value := range want {
		if !inHave[value] {
			lines = append(lines, "  only in the spec:  "+value)
		}
	}
	return strings.Join(lines, "\n")
}

// TestCloudExtensionConfigValidationIssuesNoRequest ports the TypeScript
// "does not expose the config validation operation whose only declared response
// is unsupported" case.
//
// The two SDKs answer it differently: the TypeScript client omits the method
// entirely, while this one keeps a method that refuses locally. What both
// guarantee - and what this asserts - is that no request ever reaches the
// endpoint whose only documented outcome is a 400.
func TestCloudExtensionConfigValidationIssuesNoRequest(t *testing.T) {
	t.Parallel()

	client, transport := cloudTestClient(t)
	result, err := NewKafkaConnectClient(client).ValidateConfig(
		context.Background(), "plugin/name", map[string]string{"tasks.max": "2"},
	)

	if result != nil {
		t.Errorf("result = %#v, want nil", result)
	}
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("err = %v, want the unsupported-validation error", err)
	}
	if calls := transport.Calls(); len(calls) != 0 {
		t.Errorf("captured %d requests, want none: %v", len(calls), calls)
	}
}

// TestCloudExtensionDoesNotServeTriggers ports the TypeScript
// "does not retain Triggers after their promotion to the core API" case.
//
// There is no `cloud` namespace object in Go to probe for a `triggers` key, so
// the port asserts the equivalent fact about paths: neither the contract nor
// any mapped client call reaches a triggers resource under the cloud prefix.
// Triggers are a core resource and resolve under /v1 instead.
func TestCloudExtensionDoesNotServeTriggers(t *testing.T) {
	t.Parallel()

	for _, spec := range cloudSpecOperations {
		if strings.HasPrefix(spec.path, "/triggers") {
			t.Errorf("spec operation %q still declares a cloud triggers path %q",
				spec.operationID, spec.path)
		}
	}
	for _, operation := range cloudContractOperations() {
		if strings.HasPrefix(operation.path, cloudExtensionPathPrefix+"/triggers") {
			t.Errorf("operation %q targets a cloud triggers path %q",
				operation.operationID, operation.path)
		}
	}
}

// TestCloudExtensionOperationsAreRootedAtTheExtensionPrefix asserts the whole
// mapped surface stays under /apis/cloud.sn.io/v1. Core managed-agent resources
// resolve under /v1 and must never appear here.
func TestCloudExtensionOperationsAreRootedAtTheExtensionPrefix(t *testing.T) {
	t.Parallel()

	for _, operation := range cloudContractOperations() {
		if !strings.HasPrefix(operation.path, cloudExtensionPathPrefix+"/") {
			t.Errorf("operation %q path = %q, want it under %q",
				operation.operationID, operation.path, cloudExtensionPathPrefix)
		}
	}
}

// ---------------------------------------------------------------------------
// Shared request-body helpers
//
// Several cloud operations send multipart/form-data rather than JSON: the
// config travels as an application/json part beside the optional package file.
// These helpers let the resource tests assert on those parts, and are shared
// here for the same reason the operation table is.
// ---------------------------------------------------------------------------

// cloudDecodeMultipart parses a captured multipart request into its non-file
// fields and its file parts, failing the test if the body is not multipart.
func cloudDecodeMultipart(t *testing.T, call capturedCall) (map[string]string, map[string][]byte) {
	t.Helper()
	return decodeMultipartRequest(t, &http.Request{
		Header: call.Header,
		Body:   io.NopCloser(bytes.NewReader(call.Body)),
	})
}

// cloudDecodeJSONField decodes one multipart field as a JSON object.
func cloudDecodeJSONField(t *testing.T, fields map[string]string, name string) map[string]any {
	t.Helper()

	raw, ok := fields[name]
	if !ok {
		t.Fatalf("multipart part %q is missing; parts present: %v", name, cloudFieldNames(fields))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("part %q = %q, which is not a JSON object: %v", name, raw, err)
	}
	return decoded
}

// cloudAssertJSONField asserts a multipart field holds exactly the expected
// JSON value.
func cloudAssertJSONField(t *testing.T, fields map[string]string, name string, want map[string]any) {
	t.Helper()

	got := cloudDecodeJSONField(t, fields, name)
	if cloudJSON(t, got) != cloudJSON(t, want) {
		t.Errorf("part %q = %s, want %s", name, cloudJSON(t, got), cloudJSON(t, want))
	}
}

// cloudAssertJSONFieldContains asserts a multipart field carries at least the
// expected entries. Used where the encoded struct also carries fields the
// caller never set - see TestCloudFunctionsMultipartBodies.
func cloudAssertJSONFieldContains(t *testing.T, fields map[string]string, name string, want map[string]any) {
	t.Helper()

	decoded := cloudDecodeJSONField(t, fields, name)
	for key, expected := range want {
		got, ok := decoded[key]
		if !ok {
			t.Errorf("part %q has no %q field; got %s", name, key, cloudJSON(t, decoded))
			continue
		}
		if cloudJSON(t, got) != cloudJSON(t, expected) {
			t.Errorf("part %q field %q = %s, want %s", name, key, cloudJSON(t, got), cloudJSON(t, expected))
		}
	}
}

func cloudFieldNames(fields map[string]string) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	return names
}

func cloudFileNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
}

// cloudJSON renders a value as canonical JSON, so comparisons do not depend on
// Go struct field order.
func cloudJSON(t *testing.T, value any) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshaling %#v: %v", value, err)
	}
	return string(encoded)
}
