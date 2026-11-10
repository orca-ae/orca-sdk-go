// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every public engine operation is classified independently of path coverage.
var engineOperations = []struct{ id, method, path, class, reason string }{
	{"discovery.coreVersions", "GET", "/api", "portable", ""},
	{"discovery.groups", "GET", "/apis", "portable", ""},
	{"discovery.policyGroupResources", "GET", "/apis/policy.runorca.ai/v1", "engine", ""},
	{"guardrails.effective", "GET", "/apis/policy.runorca.ai/v1/effectiveguardrails", "engine", ""},
	{"guardrails.create", "POST", "/apis/policy.runorca.ai/v1/guardrails", "engine", ""},
	{"guardrails.list", "GET", "/apis/policy.runorca.ai/v1/guardrails", "engine", ""},
	{"guardrails.get", "GET", "/apis/policy.runorca.ai/v1/guardrails/{id}", "engine", ""},
	{"guardrails.update", "POST", "/apis/policy.runorca.ai/v1/guardrails/{id}", "engine", ""},
	{"guardrails.delete", "DELETE", "/apis/policy.runorca.ai/v1/guardrails/{id}", "engine", ""},
	{"guardrails.archive", "POST", "/apis/policy.runorca.ai/v1/guardrails/{id}/archive", "engine", ""},
	{"guardrails.listTypes", "GET", "/apis/policy.runorca.ai/v1/guardrailtypes", "engine", ""},
	{"guardrails.dogwoodSchema", "GET", "/apis/policy.runorca.ai/v1/policyschemas/dogwood", "engine", ""},
	{"discovery.pricingGroupResources", "GET", "/apis/pricing.runorca.ai/v1", "engine", ""},
	{"modelPrices.list", "GET", "/apis/pricing.runorca.ai/v1/modelprices", "engine", ""},
	{"modelPrices.get", "GET", "/apis/pricing.runorca.ai/v1/modelprices/{model_id}", "engine", ""},
	{"discovery.pricingV2GroupResources", "GET", "/apis/pricing.runorca.ai/v2", "engine", ""},
	{"modelPriceRules.list", "GET", "/apis/pricing.runorca.ai/v2/modelpricerules", "engine", ""},
	{"discovery.runtimeGroupResources", "GET", "/apis/runtime.runorca.ai/v1", "engine", ""},
	{"discovery.harnesses", "GET", "/apis/runtime.runorca.ai/v1/harnesses", "engine", ""},
	{"health.healthz", "GET", "/healthz", "portable", ""},
	{"health.readyz", "GET", "/readyz", "portable", ""},
	{"agents.create", "POST", "/v1/agents", "portable", ""},
	{"agents.list", "GET", "/v1/agents", "portable", ""},
	{"agents.get", "GET", "/v1/agents/{id}", "portable", ""},
	{"agents.update", "POST", "/v1/agents/{id}", "portable", ""},
	{"agents.delete", "DELETE", "/v1/agents/{id}", "excluded", "deployment overlay removes agent deletion; archive instead"},
	{"agents.archive", "POST", "/v1/agents/{id}/archive", "portable", ""},
	{"agents.listVersions", "GET", "/v1/agents/{id}/versions", "portable", ""},
	{"environments.create", "POST", "/v1/environments", "portable", ""},
	{"environments.list", "GET", "/v1/environments", "portable", ""},
	{"environments.get", "GET", "/v1/environments/{id}", "portable", ""},
	{"environments.update", "POST", "/v1/environments/{id}", "portable", ""},
	{"environments.delete", "DELETE", "/v1/environments/{id}", "portable", ""},
	{"environments.archive", "POST", "/v1/environments/{id}/archive", "portable", ""},
	{"environments.revokeKey", "POST", "/v1/environments/{id}/revoke-key", "engine", ""},
	{"environments.rotateKey", "POST", "/v1/environments/{id}/rotate-key", "engine", ""},
	{"environments.workStats", "GET", "/v1/environments/{id}/work_stats", "engine", ""},
	{"files.create", "POST", "/v1/files", "portable", ""},
	{"files.list", "GET", "/v1/files", "portable", ""},
	{"files.get", "GET", "/v1/files/{id}", "portable", ""},
	{"files.delete", "DELETE", "/v1/files/{id}", "portable", ""},
	{"files.getContent", "GET", "/v1/files/{id}/content", "portable", ""},
	{"gitCreds.resolveCreds", "POST", "/v1/git-creds", "excluded", "deployment overlay removes nonportable Git credentials"},
	{"gitProxy.uploadPack", "POST", "/v1/git-proxy/{resourceId}/git-upload-pack", "excluded", "special JWT authentication and binary Git smart HTTP are outside the SDK resource API"},
	{"gitProxy.advertise", "GET", "/v1/git-proxy/{resourceId}/info/refs", "excluded", "special JWT authentication and binary Git smart HTTP are outside the SDK resource API"},
	{"memoryStores.create", "POST", "/v1/memory_stores", "portable", ""},
	{"memoryStores.list", "GET", "/v1/memory_stores", "portable", ""},
	{"memoryStores.get", "GET", "/v1/memory_stores/{id}", "portable", ""},
	{"memoryStores.update", "POST", "/v1/memory_stores/{id}", "portable", ""},
	{"memoryStores.delete", "DELETE", "/v1/memory_stores/{id}", "portable", ""},
	{"memoryStores.archive", "POST", "/v1/memory_stores/{id}/archive", "portable", ""},
	{"memoryStores.createMemory", "POST", "/v1/memory_stores/{id}/memories", "portable", ""},
	{"memoryStores.listMemories", "GET", "/v1/memory_stores/{id}/memories", "portable", ""},
	{"memoryStores.getMemory", "GET", "/v1/memory_stores/{id}/memories/{memory_id}", "portable", ""},
	{"memoryStores.updateMemory", "POST", "/v1/memory_stores/{id}/memories/{memory_id}", "portable", ""},
	{"memoryStores.deleteMemory", "DELETE", "/v1/memory_stores/{id}/memories/{memory_id}", "portable", ""},
	{"memoryStores.listVersions", "GET", "/v1/memory_stores/{id}/memory_versions", "portable", ""},
	{"memoryStores.getVersion", "GET", "/v1/memory_stores/{id}/memory_versions/{version_id}", "portable", ""},
	{"memoryStores.redactVersion", "POST", "/v1/memory_stores/{id}/memory_versions/{version_id}/redact", "portable", ""},
	{"sessions.create", "POST", "/v1/sessions", "portable", ""},
	{"sessions.list", "GET", "/v1/sessions", "portable", ""},
	{"sessions.get", "GET", "/v1/sessions/{id}", "portable", ""},
	{"sessions.update", "POST", "/v1/sessions/{id}", "portable", ""},
	{"sessions.delete", "DELETE", "/v1/sessions/{id}", "portable", ""},
	{"sessions.archive", "POST", "/v1/sessions/{id}/archive", "portable", ""},
	{"sessions.appendEvents", "POST", "/v1/sessions/{id}/events", "portable", ""},
	{"sessions.listEvents", "GET", "/v1/sessions/{id}/events", "portable", ""},
	{"sessions.streamEvents", "GET", "/v1/sessions/{id}/events/stream", "portable", ""},
	{"sessions.listFiles", "GET", "/v1/sessions/{id}/files", "portable", ""},
	{"sessions.getFile", "GET", "/v1/sessions/{id}/files/{file_id}", "portable", ""},
	{"sessions.deleteFile", "DELETE", "/v1/sessions/{id}/files/{file_id}", "portable", ""},
	{"sessions.getFileContent", "GET", "/v1/sessions/{id}/files/{file_id}/content", "portable", ""},
	{"outcomes.get", "GET", "/v1/sessions/{id}/outcome", "portable", ""},
	{"sessions.attachResource", "POST", "/v1/sessions/{id}/resources", "portable", ""},
	{"sessions.listResources", "GET", "/v1/sessions/{id}/resources", "portable", ""},
	{"sessions.getResource", "GET", "/v1/sessions/{id}/resources/{resource_id}", "portable", ""},
	{"sessions.updateResource", "POST", "/v1/sessions/{id}/resources/{resource_id}", "portable", ""},
	{"sessions.detachResource", "DELETE", "/v1/sessions/{id}/resources/{resource_id}", "portable", ""},
	{"sessions.listThreads", "GET", "/v1/sessions/{id}/threads", "portable", ""},
	{"sessions.retrieveThread", "GET", "/v1/sessions/{id}/threads/{thread_id}", "portable", ""},
	{"sessions.archiveThread", "POST", "/v1/sessions/{id}/threads/{thread_id}/archive", "portable", ""},
	{"sessions.listThreadEvents", "GET", "/v1/sessions/{id}/threads/{thread_id}/events", "portable", ""},
	{"sessions.interruptThread", "POST", "/v1/sessions/{id}/threads/{thread_id}/interrupt", "engine", ""},
	{"sessions.streamThread", "GET", "/v1/sessions/{id}/threads/{thread_id}/stream", "portable", ""},
	{"skills.create", "POST", "/v1/skills", "portable", ""},
	{"skills.list", "GET", "/v1/skills", "portable", ""},
	{"skills.get", "GET", "/v1/skills/{id}", "portable", ""},
	{"skills.delete", "DELETE", "/v1/skills/{id}", "portable", ""},
	{"skills.createVersion", "POST", "/v1/skills/{id}/versions", "portable", ""},
	{"skills.listVersions", "GET", "/v1/skills/{id}/versions", "portable", ""},
	{"skills.getVersion", "GET", "/v1/skills/{id}/versions/{version}", "portable", ""},
	{"skills.deleteVersion", "DELETE", "/v1/skills/{id}/versions/{version}", "portable", ""},
	{"skills.getVersionContent", "GET", "/v1/skills/{id}/versions/{version}/content", "portable", ""},
	{"triggers.create", "POST", "/v1/triggers", "portable", ""},
	{"triggers.list", "GET", "/v1/triggers", "portable", ""},
	{"triggers.get", "GET", "/v1/triggers/{id}", "portable", ""},
	{"triggers.update", "POST", "/v1/triggers/{id}", "portable", ""},
	{"triggers.delete", "DELETE", "/v1/triggers/{id}", "portable", ""},
	{"triggers.pause", "POST", "/v1/triggers/{id}/pause", "portable", ""},
	{"triggers.sessions", "GET", "/v1/triggers/{id}/sessions", "portable", ""},
	{"triggers.unpause", "POST", "/v1/triggers/{id}/unpause", "portable", ""},
	{"vaults.create", "POST", "/v1/vaults", "portable", ""},
	{"vaults.list", "GET", "/v1/vaults", "portable", ""},
	{"vaults.get", "GET", "/v1/vaults/{id}", "portable", ""},
	{"vaults.update", "POST", "/v1/vaults/{id}", "portable", ""},
	{"vaults.delete", "DELETE", "/v1/vaults/{id}", "portable", ""},
	{"vaults.archive", "POST", "/v1/vaults/{id}/archive", "portable", ""},
	{"vaults.listCredentials", "GET", "/v1/vaults/{vault_id}/credentials", "portable", ""},
	{"vaults.createCredential", "POST", "/v1/vaults/{vault_id}/credentials", "portable", ""},
	{"vaults.getCredential", "GET", "/v1/vaults/{vault_id}/credentials/{credential_id}", "portable", ""},
	{"vaults.updateCredential", "POST", "/v1/vaults/{vault_id}/credentials/{credential_id}", "portable", ""},
	{"vaults.deleteCredential", "DELETE", "/v1/vaults/{vault_id}/credentials/{credential_id}", "portable", ""},
	{"vaults.archiveCredential", "POST", "/v1/vaults/{vault_id}/credentials/{credential_id}/archive", "portable", ""},
	{"vaults.mcpOauthValidateCredential", "POST", "/v1/vaults/{vault_id}/credentials/{credential_id}/mcp_oauth_validate", "portable", ""},
}

func TestEngineOperationCoverage(t *testing.T) {
	data, err := os.ReadFile("openapi/managed-agents-engine.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile("^      operationId: (.+)$")
	declared := map[string]string{}
	path, method := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "  /") {
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
		}
		for _, verb := range []string{"get", "post", "put", "patch", "delete"} {
			if line == "    "+verb+":" {
				method = strings.ToUpper(verb)
			}
		}
		if match := pattern.FindStringSubmatch(line); match != nil {
			declared[match[1]] = method + " " + path
		}
	}
	if len(declared) != 114 {
		t.Fatalf("engine operation count = %d, want 114; review contract alignment", len(declared))
	}
	built := sdkPaths(t, "v1/", "apis/policy.", "apis/pricing.", "apis/runtime.")
	counts := map[string]int{}
	for _, op := range engineOperations {
		if declared[op.id] != op.method+" "+op.path {
			t.Errorf("missing, duplicate or changed operation %s: %s", op.id, declared[op.id])
		}
		delete(declared, op.id)
		counts[op.class]++
		if op.class == "excluded" {
			if op.reason == "" {
				t.Errorf("unjustified exclusion %s", op.id)
			}
			continue
		}
		if op.class != "portable" && op.class != "engine" {
			t.Errorf("unknown classification %s", op.class)
		}
		if strings.HasPrefix(op.path, "/v1/") || strings.HasPrefix(op.path, "/apis/") {
			template := specPlaceholder.ReplaceAllString(op.path, "{}")
			if _, ok := built[template]; !ok {
				t.Errorf("%s %s has no SDK path (%s)", op.method, op.path, op.id)
			}
		}
	}
	if len(declared) != 0 {
		t.Errorf("unclassified engine operations: %v", declared)
	}
	t.Logf("engine coverage: %v", counts)
}
