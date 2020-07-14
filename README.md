# `mergermarket/cdflow2-config-acuris`

This container is for teams using cdflow2 to deploy services within Acuris - i.e. Acuris staff members and partners building services for Acuris infrastructure. If this doesn't appy to you, consult the [cdflow2 documentation](https://developer-preview.acuris.com/opensource/cdflow2/) to see if there is a config container that will work for your use-case (or how to build one).

## Usage

`cdflow.yaml`:

```yaml
version: 2
config:
  image: mergermarket/cdflow2-config-acuris
  params:
    account_prefix: myprefix
    team: my-team-name
terraform:
  image: hashicorp/terraform
```

### Parameters

#### `account_prefix`

Each team in Acuris deploys to a pair of AWS accounts. The "live" environment is deployed
in an account ending in "prod", and all pre-live environments are deployed in an account
ending in "dev". This parameter contains the prefix that is applied to both of these account
aliases. For example if the `account_prefix` is `"mmg"` then the account aliases would be
`"mmgdev"` and `"mmgprod"`. Usually a team will only hae permissions to manage infrastructure in one of these account pairs, so check with the Acuris Platform Team which account pair (and hence account_prefix) you should use.

#### `team`

This is the short name for your team - i.e. the name of your team's Jenkins folder as it appears in a Jenkins URL. Usually a team's deploy credentials will only have access to write release info, terraform state, etc to paths prefixed with the team's name, so this will need to be right for your pipeline to work.

## What this config plugin provides

### Release metadata

An additional `team` key is added to the `release` terraform map variable so that your terraform code can use it for tagging resources.

### ECR builds

If you have build(s) configured in your `cdflow.yaml` that advertise the need for `"ecr"` (e.g. [mergermarket/cdflow-build-docker-ecr](https://hub.docker.com/r/mergermarket/cdflow2-build-docker-ecr)), then this container will provision an ECR repository.

When providing configuration for the release it will:

* Ensure a ECR repository exists in the `eu-west-1` region in the `acurisrelease` account, following the naming convention `<team>-<component>` (e.g. `"myteam-myservice"`). The team prefix is important since it will only have permission to create repositories with this prefix.
* Ensure that [image scanning](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-scanning.html) is turned on (scan on push).
* Ensure that [image tag mutability](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-tag-mutability.html) is set to `IMMUTABLE`.
* Ensure a [lifecycle policy](https://docs.aws.amazon.com/AmazonECR/latest/userguide/LifecyclePolicies.html) exists that retains the 50 newest images for each build prefix.
* Provide an `ECR_REPOSITORY` environment variable containing the repository address.
* Provide AWS credentials in the environment for the build for the `<team>-deploy` IAM role in the `acurisrelease` account, which is authorised to push docker images to the repository.
* Provide the AWS region (`"eu-west-1"`) via `AWS_REGION` and `AWS_DEFAULT_REGION` environment varialbes to the build container.

### Lambda builds

WARNING: lambda support is work in progress - this is subject to change.

If you have build(s) configured in your `cdflow.yaml` that advertise the need for `"ecr"` (e.g. [mergermarket/cdflow-build-docker-ecr](https://hub.docker.com/r/mergermarket/cdflow2-build-docker-ecr)), then this will:

* Provide a `LAMBDA_BUCKET` environment variable to the build, containing the name of the S3 bucket where lambdas should be stored.
* Provide AWS credentials in the environment for the build for the `<team>-deploy` IAM role in the `acurisrelease` account, which is authorised to upload lambdas to the lambda bucket.
* Provide the AWS region (`"eu-west-1"`) via `AWS_REGION` and `AWS_DEFAULT_REGION` environment varialbes to the build container.

### Storing the release

At the end of the release the config container is invoked to persist the release, including release metadata and terraform providers and modules.

### Retrieving the release and deployment enironment

`cdflow2` commands that require terraform to be configured (e.g. `deploy`, `destroy`, `shell`) use the config container to retrieve the release from S3. The following is provided:

* AWS credentials for the `<team>-deploy` IAM role in the relevant deployment account (i.e. `<account_prefix>prod` for the `live` environment, `<account_prefix>dev` otherwise).
* The AWS region via the `AWS_DEFAULT_REGION` environment variable (currently always `"eu-west-1"`).
* As with all terraform config plugins, terraform map variables for each build with the build metadata, as well as a general `release` map with an additional `team` key (both persisted in the release).
