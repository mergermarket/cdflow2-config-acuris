This container is for teams using cdflow2 to deploy services within Acuris.

Usage in `cdflow.yaml`:

```yaml
version: 2
team: my-team-name
config:
  image: mergermarket/cdflow2-config-acuris
  accountprefix: myprefix
terraform:
  image: hashicorp/terraform
```
