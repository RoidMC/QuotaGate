# QuotaGate Backend

Headless Lightway & Fast AI API Router

> ⚠️ **Warning**
>
> It is strictly prohibited for this project to be in the same database as KexCore IAM. KexCore IAM for this project only shares the same architecture and part of the code, and any database damage or failure caused by connecting to the same database will be the responsibility of the individual

## KexCore Architecture

QuotaGate Backend is built on the **KexCore** architecture, a modular backend framework designed for high-performance API routing and service orchestration.

> ⚠️ **Experimental Feature Warning**
>
> QuotaGate depends on `github.com/lestrrat-go/jwx/v4`, which requires the Go experimental feature `encoding/json/v2`.
> This feature is **not yet stable** and may change in future Go releases.
>
> You **must** enable it before building or running QuotaGate:
>
> ```bash
> export GOEXPERIMENT=jsonv2
> go build ./...
> ```
>
> For VS Code / gopls, add the following to `.vscode/settings.json`:
>
> ```json
> {
>   "go.toolsEnvVars": {
>     "GOEXPERIMENT": "jsonv2"
>   }
> }
> ```
>
> If `encoding/json/v2` is removed or changed in a future Go version, this project may need to downgrade jwx to v3.