# Disposable Azure SSE validation

Experimental harness only. The production Dockerfile still builds `cmd/web`.
Do not enable these fixture endpoints in the production app.

Build `cmd/streamprobe` for Linux amd64 into `probe`, then use `Dockerfile.probe`.
The command requires `DB_URL`, `DB_KEY`, a random `PROBE_SECRET` of at least32
characters and a unique `PROBE_OWNER` prefix. Use a temporary ACA app with the
existing small Consumption allocation and min0/max1. Use only disposable data.
`PROBE_RECHECK_SECONDS` defaults15; the revised experiment used60, independent of
15-second heartbeats. All control endpoints require the random header secret.

The browser scripts use Playwright and installed macOS Chrome. Their private
configuration is read from `/tmp/camplist-probe-access.json` containing `secret`
and `owner`; never commit this file. Adjust temporary host, registry, resource
group, executable and output paths for a new environment. Scripts reference the
specific disposable app used for the documented investigation, not production.

1. Run `experiment.cjs` and `rollout.cjs` concurrently after provisioning. The
   experiment waits more than ten minutes and signals the rollout via temporary
   marker files. Delete old marker/result files before a new run.
2. Run `followup.cjs` after the first run to check the revised idle cost, public
   invitation rejoin, conflict handling and account mismatch.
3. `scale-cold.cjs` waits for followup completion, observes replicas through ARM,
   tests cold start and deletes fixture records. It deliberately avoids app HTTP
   traffic while waiting for scale0.
4. Independently verify cleanup; delete the temporary app and `stream-probe`
   image repository, remove private files, and retain only sanitized results.

Login uses signed fixture identities, not Google OAuth. Short expiry and mobile
visibility transitions are controlled simulations. See the experiment report for
coverage and limitations; do not describe them as physical-device testing.
