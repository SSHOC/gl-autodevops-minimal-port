A port of the Gitlab AutoDevOps Workflow to GitHub Actions
==========================================================

This repository contains [reusable workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows) that try to achieve the same result as the

* build
* custom test and
* deploy

[stages of the Gitlab AutoDevOps](https://docs.gitlab.com/ee/topics/autodevops/stages.html) workflow.

Usage
-----

To use this you add a `starter.yaml` to `.github/workflows` in your GitHub repository with something like this:

```yaml
name: workflows starter
# env: is empty, see setup-env and the outputs there
on:
  push: {}
  workflow_dispatch: {}
jobs:
  setup_workflow_env:
    runs-on: ubuntu-latest
# Optionally specify the environment that should be used on this branch
    # environment: review/dev
    outputs:
# It seems you have to specify the environment twice (passed to reusable workflow)
# as there is no way yet to get the active environment
      # environment: review/dev
# or see the switch on ref_name script below
      environment: ${{ steps.get_environment_from_git_ref.outputs.environment }}
      image_name: your-image-name
      registry_root: ghcr.io/${{ github.repository }}/
      default_port: "5000"
      submodules: "true"
#      herokuish_base_image: ghcr.io/acdh-oeaw/herokuish-for-cypress/main:latest-22
      APP_NAME: your-app-name
# This together with the branch name is also used as the namespace to deploy to
      APP_ROOT: "/"     
      # SERVICE_ID: "99999" # Better use GtiHub environment variables for this
      # PUBLIC_URL: "https://some-stuff.acdh-ch-dev.oeaw.ac.at" # Better use GitHub environment variables for this
      # POSTGRES_ENABLED: "false" # needs to be set to true to enable a postgres db installed next to the deployed app
# You should not need to have to change anything below this line
#-----------------------------------------------------------------------------------------------------
    steps:
      - name: Get environment from git ref
        id: get_environment_from_git_ref
        run: |
          echo "Running on branch ${{ github.ref_name }}"
          if [ "${{ github.ref }}" = "refs/heads/main" ]; then
            echo "environment=production"
            echo "environment=production" >> $GITHUB_OUTPUT
          else
            echo "environment=review/${{ github.ref_name }}"
            echo "environment=review/${{ github.ref_name }}" >> $GITHUB_OUTPUT
          fi      
  _1:
    needs: [setup_workflow_env]
    uses:  InTaVia/gl-autodevops-minimal-port/.github/workflows/build-cnb-and-push-to-registry.yaml@main
    secrets: inherit
# if you run this outside of of an org that provides KUBE_CONFIG etc as a secret, you need to specify every secret you want to pass by name
    with:
      environment: ${{ needs.setup_workflow_env.outputs.environment }}
      registry_root: ${{ needs.setup_workflow_env.outputs.registry_root }}
      image_name: ${{ needs.setup_workflow_env.outputs.image_name }}
      source_image: ${{ needs.setup_workflow_env.outputs.source_image }}
      default_port: ${{ needs.setup_workflow_env.outputs.default_port }}
      submodules: ${{ needs.setup_workflow_env.outputs.submodules }}
  _2:
    needs: [setup_workflow_env]
    uses:  InTaVia/gl-autodevops-minimal-port/.github/workflows/herokuish-tests-db-url.yaml@main
    secrets: inherit
# if you run this outside of InTaVia yo uneed to specify every secret you want to pass by name
    with:
      environment: ${{ needs.setup_workflow_env.outputs.environment}}
      registry_root: ${{ needs.setup_workflow_env.outputs.registry_root }}
      image_name: ${{ needs.setup_workflow_env.outputs.image_name }}
      source_image: ${{ needs.setup_workflow_env.outputs.source_image }}
      default_port: ${{ needs.setup_workflow_env.outputs.default_port }}
      herokuish_base_image: ${{ needs.setup_workflow_env.outputs.herokuish_base_image }}
      POSTGRES_ENABLED: ${{ needs.setup_workflow_env.outputs.POSTGRES_ENABLED }}
      submodules: ${{ needs.setup_workflow_env.outputs.submodules }}
  _3:
    needs: [setup_workflow_env, _1, _2]
    uses: InTaVia/gl-autodevops-minimal-port/.github/workflows/deploy-cluster-2.yml@main
    secrets: inherit
# if you run this outside of InTaVia yo uneed to specify every secret you want to pass by name
#      KUBE_CONFIG: ${{ secrets.KUBE_CONFIG }}
#      KUBE_INGRESS_BASE_DOMAIN: ${{ secrets.KUBE_INGRESS_BASE_DOMAIN }}
#      POSTGRES_USER: ${{ secrets.POSTGRES_USER }}
#      POSTGRES_PASSWORD: ${{ secrets.POSTGRES_PASSWORD }}
#      POSTGRES_DB: ${{ secrets.POSTGRES_DB }}
#      K8S_SECRET_A_VAR_NAME: ${{  }}
    with:
      environment: ${{ needs.setup_workflow_env.outputs.environment}}
      DOCKER_TAG: ${{ needs.setup_workflow_env.outputs.registry_root }}${{ needs.setup_workflow_env.outputs.image_name }}
      APP_NAME: ${{ needs.setup_workflow_env.outputs.APP_NAME }}-${{ needs.setup_workflow_env.outputs.environment }}
      APP_ROOT: ${{ needs.setup_workflow_env.outputs.APP_ROOT }}
      SERVICE_ID: ${{ needs.setup_workflow_env.outputs.SERVICE_ID }}
      PUBLIC_URL: ${{ needs.setup_workflow_env.outputs.PUBLIC_URL }}
      POSTGRES_ENABLED: ${{ needs.setup_workflow_env.outputs.POSTGRES_ENABLED == 'true'}}
      default_port: "${{ needs.setup_workflow_env.outputs.default_port}}"
      submodules: ${{ needs.setup_workflow_env.outputs.submodules }}
```

You can pass many parameters variables like in gitlab or use GitHub's special read protected write only secrets.
You can also use environments for having different parameters.
For example `KUBE_NAMESPACE` or `HELM_UPGRADE_EXTRA_ARGS` can be set as project or environment variables.
Deployment specific variables like `KUBE_INGRESS_BASE_DOMAIN` need to be set on the project level.
_Note_: At least one variable and one secret need to be set on the project level else `deploy.yaml` will end with an error.

Variables and Secrets
---------------------

GitHub has two ways of storing data with a repository but not in the gitted code:
* Secrets are meant for data that is to be kept secret all as much as possible  
  Examples would be:
  * Database passwords
  * API secrets
  * Maybe API Keys/Access IDs
  * Access token
  * Even encoded files such as a Kubernetes configuration
* Variables are a newer edition, that do provide a means to store some additional data that can be publicly available  
  Examples would be:
  * the public URL of a deployment
  * an ID of a deployment
  * the K8s namespace the deployment uses
  * API Keys/Access IDs 

Also the same mechanism as in gl is implemented to pass Secrets and Variables to the build process and the running deployment (as a K8s Secret).

Variables and secrets can be set on three levels in GitHub:
1. Organization level (Org)
2. Repository level (Repo)
3. Environment level (Env)

A Variable or Secret in a higher level overrides a Variable or Secret with the same name in a lower level.

_Note_: GitHub Environment Variables are not automaticall Workflow environment variables (vars context vs. env context)

|Name|Required|Type|Level|Description|
|----|:------:|----|:---:|-----------|
|KUBE_CONFIG|:white_check_mark:|Secret|Org|base64 encoded K8s config file. Usually set at the Org level and shared by all (public) repositories.
|C2_KUBE_CONFIG|:white_check_mark:|Secret|Org|If you deploy using the workflow for the second cluster the C2_ variant is used
|KUBE_INGRESS_BASE_DOMAIN|:white_check_mark:|Variable|Org/Repo/Env|The DNS suffix used when generating URLs for the service
|C2_KUBE_INGRESS_BASE_DOMAIN|:white_check_mark:|Variable|Org/Repo/Env|If you deploy using the workflow for the second cluster the C2_ variant is used
|KUBE_NAMESPACE|:white_check_mark:|Variable|Repo/Env|The K8s namespace the deployment should be installed to
|PUBLIC_URL|:white_check_mark:|Variable|Env|The URI that should be configured for access to the service
|SERVICE_ID|:white_check_mark:|Variable|Env|A K8s label ID is attached to the workload/deployment with this value (usually a number)
|POSTGRES_ENABLED||Variable|Repo/Env|Boolean that determines if a PostgreSQL database is installed with the deployment but using a separate helm chart. Default is false.
|POSTGRES_VERSION||Variable|Repo/Env|Version ([tag](https://hub.docker.com/r/bitnami/postgresql/tags)) of PostgreSQL to deploy. Default is 9.6.16 (for historical gl reasons)
|POSTGRES_HOST||Variable|Repo/Env|Hostname of an external PostgreSQL service
|POSTGRES_USER||Variable|Env|Username for the PostgreSQL database. Will be configured for the new PostgreSQL deployment if POSTGRES_ENABLED is true
|POSTGRES_PASSWORD||Secret|Env|Password for the PostgreSQL database. Will be configured for the new PostgreSQL deployment if POSTGRES_ENABLED is true
|POSTGRES_DB||Variable|Env|Name of the PostgreSQL database to use. Will be created in the new PostgreSQL deployment if POSTGRES_ENABLED is true
|DATABASE_URL||Secret|Env|Credentials for a database passed to the running workload in a URL form (`db_type://username:password@db_host/db_name`). This is automatically genereated for PostgreSQL database installed with the deployment. Store as a Secret as it usually contains the password.
|HELM_UPGRADE_EXTRA_ARGS||Variable|Repo/Env|Used to set a few values from the Helm charts value.yaml using `--set` command line parameters to `helm`. If you have to set more or nested values better use a `auto-deploy-values.yaml` file in the git repository. Store as a Secret if you `--set` sensitive information (not recommended)
|K8S_SECRET_`<ENV_VAR_NAME>`||Variable/Secret|Repo/Env|Passes `ENV_VAR_NAME` to the build process and to the running workload using a K8s secret
|LC_K8S_SECRET_`<ENV_VAR_NAME>`||Variable/Secret|Repo/Env|Passes `env_var_name` to the build process and to the running workload using a K8s secret. GitHub does not allow Variables or Secrets to contain lower case letters (yet)|

_Note_: Some of the settings stored in variables above are also recognized as Secrets for legacy reasons. There is however no point in using them like this. Also some of the variables can be set in the suggested `starter.yaml`. This is only a useful place to set such variables if you don't work with environments.

Example DATABASE_URLs:
* `postgres://deployment:abcd098ABCD:5432@dbserver.example.org/deployment`
* `postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_HOST:5432/$POSTGRES_DB`
* `mariadb://deployment:abcd098ABCD:3306@dbserver.example.org/deployment`
* `mysql://deployment:abcd098ABCD:3306@dbserver.example.org/deployment`

[For the POSTGRES_ variables see also the gl docs.](https://docs.gitlab.com/ee/topics/autodevops/cicd_variables.html#database-variables)

Customizing the deployment
--------------------------

The `auto-deploy-app` helm chart from gl we use can be tweaked in many ways with a [values.yaml](.github\auto-deploy-app\values.yaml) file.

If you store your settings as `.github/auto-deploy-values.yaml` or `.gitlab/auto-deploy-values.yaml` in the root of your repoitory it will be picked up by the deployment script and used to customize the `auto-deploy-app` chart.

If you need to further customize deployment (like deploying an extra service like solr with your application) you can store a bundled helm chart in a directory `chart` in your repository and that will be used instead of the generic `auto-deploy-app` chart from this repository.

[See also the gl documentation.](https://docs.gitlab.com/ee/topics/autodevops/customize.html#custom-helm-chart)

TODO
----

Nothing right now

Source of the auto-deploy-app
-----------------------------

The [auto-deploy-app](https://gitlab.com/gitlab-org/cluster-integration/auto-deploy-image/-/tree/master/assets/auto-deploy-app) helm chart is part of the [Gitlab cluster-integration auto-deploy-image repository](https://gitlab.com/gitlab-org/cluster-integration/auto-deploy-image)

This helm chart should be updated onco in a while.

_Note:_ At least one Secret and one Variable is required for the workflows in this repository to work. Usually at least a K8s config as secret and a KUBE_INGRESS_BASE_DOMAIN are set so this limitation is rarely encountered.