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
jobs:
  setup_workflow_env:
    runs-on: ubuntu-latest
    outputs: 
      image_tagged: voice-fe
      registry_root: ghcr.io/acdh-oeaw/
      default_port: "5000"
      source_image: tmp-cnb-image
      herokuish_base_image: ghcr.io/acdh-oeaw/herokuish-for-cypress/main
      APP_NAME: voice-fe
# This together with the branch name is also used as the namespace to deploy to
      APP_ROOT: "/"
      SERVICE_ID: "18319"
      PUBLIC_URL: https://github-voice-fe-test-dev.acdh-cluster.arz.oeaw.ac.at
      POSTGRES_ENABLED: "false"
    steps:
      - run: "/bin/true"      
  _1:
    needs: [setup_workflow_env]
    uses: acdh-oeaw/gl-autodevops-minimal-port/.github/workflows/build-cnb-and-push-to-registry.yaml@main  
    with:
      registry_root: ${{ needs.setup_workflow_env.outputs.registry_root }}
      image_tagged: ${{ needs.setup_workflow_env.outputs.image_tagged }}
      source_image: ${{ needs.setup_workflow_env.outputs.source_image }}
      default_port: ${{ needs.setup_workflow_env.outputs.default_port }}
  _2:
    needs: [setup_workflow_env]
    uses: acdh-oeaw/gl-autodevops-minimal-port/.github/workflows/herokuish-tests-db-url.yaml@main
    with:
      registry_root: ${{ needs.setup_workflow_env.outputs.registry_root }}
      image_tagged: ${{ needs.setup_workflow_env.outputs.image_tagged }}
      source_image: ${{ needs.setup_workflow_env.outputs.source_image }}
      default_port: ${{ needs.setup_workflow_env.outputs.default_port }}
      herokuish_base_image: ${{ needs.setup_workflow_env.outputs.herokuish_base_image }}
      POSTGRES_ENABLED: ${{ needs.setup_workflow_env.outputs.POSTGRES_ENABLED }}
  _3:
    needs: [setup_workflow_env, _1, _2]
    uses: acdh-oeaw/gl-autodevops-minimal-port/.github/workflows/deploy.yml@main
    secrets:
      ACDH_KUBE_CONFIG: ${{ secrets.ACDH_KUBE_CONFIG }}
      POSTGRES_USER: ${{ secrets.POSTGRES_USER }}
      POSTGRES_PASSWORD: ${{ secrets.POSTGRES_PASSWORD }}
      POSTGRES_DB: ${{ secrets.POSTGRES_DB }}
    with:
      DOCKER_TAG: ${{ needs.setup_workflow_env.outputs.registry_root }}${{ needs.setup_workflow_env.outputs.image_tagged }}/${{ github.ref_name }}
      APP_NAME: ${{ needs.setup_workflow_env.outputs.APP_NAME }}
      APP_ROOT: ${{ needs.setup_workflow_env.outputs.APP_ROOT }}
      SERVICE_ID: ${{ needs.setup_workflow_env.outputs.SERVICE_ID }}
      PUBLIC_URL: ${{ needs.setup_workflow_env.outputs.PUBLIC_URL }}
      POSTGRES_ENABLED: ${{ needs.setup_workflow_env.outputs.POSTGRES_ENABLED == 'true'}}
```

TODO
----

* Pass environment variables to production pod from repo
* Pass environment variables to build and/or test build containers
* Capture the test artefacts (e.g. cypress videos). Also on errors.