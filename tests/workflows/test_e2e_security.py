# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0
"""Fail closed if private-source E2E regains a PR execution path."""
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOW = (ROOT / '.github/workflows/e2e-managed-agents.yml').read_text()
GUARD = """github.ref == 'refs/heads/main' &&
      (github.event_name == 'push' || github.event_name == 'schedule' || github.event_name == 'workflow_dispatch')"""


class E2ESecurity(unittest.TestCase):
    def test_no_pr_or_external_trigger(self):
        triggers = WORKFLOW.split('permissions:', 1)[0]
        for event in ('pull_request', 'pull_request_target', 'workflow_run', 'repository_dispatch'):
            self.assertNotIn(event + ':', triggers)
        for event in ('push:', 'schedule:', 'workflow_dispatch:'):
            self.assertIn(event, triggers)

    def test_both_jobs_gate_event_and_ref_before_credentials(self):
        for job in ('secret-gate', 'e2e'):
            content = WORKFLOW.split('\n  ' + job + ':\n')[1]
            header = content.split('    steps:', 1)[0]
            self.assertIn('    if: >-\n', header)
            self.assertIn(GUARD, header)
        self.assertIn("needs.secret-gate.outputs.enabled == 'true' &&", WORKFLOW)

    def test_immutable_checkout_without_persisted_credentials(self):
        self.assertIn('ref: ${{ github.sha }}\n          persist-credentials: false', WORKFLOW)
        self.assertNotIn('pull_request.head', WORKFLOW)

    def test_fresh_runner_and_no_pr_cache(self):
        self.assertNotIn('vars.E2E_RUNNER', WORKFLOW)
        self.assertNotIn('cache: true', WORKFLOW)
        self.assertNotIn('actions/cache', WORKFLOW)
        self.assertIn('cache: false', WORKFLOW)

    def test_secretless_pr_compilation_coverage_remains(self):
        lint = (ROOT / 'scripts/lint').read_text()
        self.assertIn('go vet -tags e2e ./...', lint)
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        self.assertIn('  pull_request:', ci)
        self.assertIn('run: ./scripts/lint', ci)
        self.assertNotIn('secrets.', ci)


if __name__ == '__main__':
    unittest.main()
